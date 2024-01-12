//
// Copyright (c) 2019-2024 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package storage

import (
	"errors"
	"fmt"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	devfileConstants "github.com/devfile/devworkspace-operator/pkg/library/constants"
	containerlib "github.com/devfile/devworkspace-operator/pkg/library/container"
	nsconfig "github.com/devfile/devworkspace-operator/pkg/provision/config"
)

func getPVCSpec(name, namespace string, storageClass *string, size resource.Quantity) (*corev1.PersistentVolumeClaim, error) {

	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": size,
				},
			},
			StorageClassName: storageClass,
		},
	}, nil
}

// isEphemeral evaluates is volume component is configured as ephemeral
func isEphemeral(volume *dw.VolumeComponent) bool {
	return volume.Ephemeral != nil && *volume.Ephemeral
}

// needsStorage returns true if storage will need to be provisioned for the current workspace. Note that ephemeral volumes
// do not need to provision storage
func needsStorage(workspace *dw.DevWorkspaceTemplateSpec) bool {
	projectsVolumeIsEphemeral := false
	for _, component := range workspace.Components {
		if component.Volume != nil {
			// If any non-ephemeral volumes are defined, we need to mount storage
			if !isEphemeral(component.Volume) {
				return true
			}
			if component.Name == devfileConstants.ProjectsVolumeName {
				projectsVolumeIsEphemeral = isEphemeral(component.Volume)
			}
		}
	}
	if projectsVolumeIsEphemeral {
		// No non-ephemeral volumes, and projects volume mount is ephemeral, so all volumes are ephemeral
		return false
	}
	// Implicit projects volume is non-ephemeral, so any container that mounts sources requires storage
	return containerlib.AnyMountSources(workspace.Components)
}

func syncCommonPVC(namespace string, config *v1alpha1.OperatorConfiguration, clusterAPI sync.ClusterAPI) (*corev1.PersistentVolumeClaim, error) {
	namespacedConfig, err := nsconfig.ReadNamespacedConfig(namespace, clusterAPI)
	if err != nil {
		return nil, fmt.Errorf("failed to read namespace-specific configuration: %w", err)
	}
	pvcSize := *config.Workspace.DefaultStorageSize.Common
	if namespacedConfig != nil && namespacedConfig.CommonPVCSize != "" {
		pvcSize, err = resource.ParseQuantity(namespacedConfig.CommonPVCSize)
		if err != nil {
			return nil, err
		}
	}

	pvc, err := getPVCSpec(config.Workspace.PVCName, namespace, config.Workspace.StorageClassName, pvcSize)
	if err != nil {
		return nil, err
	}
	currObject, err := sync.SyncObjectWithCluster(pvc, clusterAPI)
	switch t := err.(type) {
	case nil:
		break
	case *sync.NotInSyncError:
		return nil, &dwerrors.RetryError{
			Message: fmt.Sprintf("Updated %s PVC on cluster", pvc.Name),
		}
	case *sync.UnrecoverableSyncError:
		return nil, &dwerrors.FailError{
			Message: fmt.Sprintf("Failed to sync %s PVC to cluster", pvc.Name),
			Err:     t.Cause,
		}
	default:
		return nil, err
	}

	currPVC, ok := currObject.(*corev1.PersistentVolumeClaim)
	if !ok {
		return nil, errors.New("tried to sync common PVC to cluster but did not get a PVC back")
	}
	// TODO: Does not work for WaitFirstConsumer storage type; needs to be improved.
	// if currPVC.Status.Phase != corev1.ClaimBound {
	// 	return nil, &NotReadyError{
	// 		Message: "Common PVC is not bound to a volume",
	// 	}
	// }
	return currPVC, nil
}

// addEphemeralVolumesFromWorkspace adds emptyDir volumes for all ephemeral volume components required for a devworkspace.
// This includes any volume components marked with the ephemeral field, including projects.
// Returns a ProvisioningError if any ephemeral volume cannot be parsed (e.g. cannot parse size for kubernetes)
func addEphemeralVolumesFromWorkspace(workspace *common.DevWorkspaceWithConfig, podAdditions *v1alpha1.PodAdditions) error {
	_, ephemeralVolumes, projectsVolume := getWorkspaceVolumes(workspace)
	_, err := addEphemeralVolumesToPodAdditions(podAdditions, ephemeralVolumes)
	if err != nil {
		return &dwerrors.FailError{Message: "Failed to add ephemeral volumes to workspace", Err: err}
	}
	if projectsVolume != nil && isEphemeral(projectsVolume.Volume) {
		if _, err := addEphemeralVolumesToPodAdditions(podAdditions, []dw.Component{*projectsVolume}); err != nil {
			return &dwerrors.FailError{Message: "Failed to add projects volume to workspace", Err: err}
		}
	}
	return nil
}

// addEphemeralVolumesToPodAdditions adds emptyDir volumes to podAdditions for each volume in workspaceVolumes.
// Returns a non-nil error if the size field of a volume is unparseable; otherwise, the list of k8s volumes that
// were added are returned.
func addEphemeralVolumesToPodAdditions(podAdditions *v1alpha1.PodAdditions, workspaceVolumes []dw.Component) (addedVolumes []corev1.Volume, err error) {
	for _, component := range workspaceVolumes {
		if component.Volume == nil {
			continue
		}
		vol := corev1.Volume{
			Name: component.Name,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
		if component.Volume.Size != "" {
			sizeResource, err := resource.ParseQuantity(component.Volume.Size)
			if err != nil {
				return nil, fmt.Errorf("failed to parse size for Volume %s: %w", component.Name, err)
			}
			vol.EmptyDir.SizeLimit = &sizeResource
		}
		podAdditions.Volumes = append(podAdditions.Volumes, vol)
		addedVolumes = append(addedVolumes, vol)
	}
	return addedVolumes, nil
}

// getWorkspaceVolumes returns all volumes defined in the DevWorkspace, separated out into persistent volumes, ephemeral
// volumes, and the projects volume, which must be handled specially. If the workspace does not define a projects volume,
// the returned value is nil.
func getWorkspaceVolumes(workspace *common.DevWorkspaceWithConfig) (persistent, ephemeral []dw.Component, projects *dw.Component) {
	for idx, component := range workspace.Spec.Template.Components {
		if component.Volume == nil {
			continue
		}
		if component.Name == devfileConstants.ProjectsVolumeName {
			projects = &workspace.Spec.Template.Components[idx]
			continue
		}
		if isEphemeral(component.Volume) {
			ephemeral = append(ephemeral, component)
		} else {
			persistent = append(persistent, component)
		}
	}
	return persistent, ephemeral, projects
}

// processProjectsVolume handles the special case of the projects volume, for which there are four possibilities:
//  1. The projects volume is not needed for the workspace (no component has mountSources: true)
//  2. The projects volume is needed but not defined in the devfile. This is the usual case, as the projects volume
//     is implicitly defined by mountSources
//  3. The projects volume is explicitly defined in the workspace, as a regular volume.
//  4. The projects volume is explicitly defined in the workspace as an ephemeral volume.
//
// To handle these cases, this function returns the projects component, if it is defined explictly (covering cases 3 and 4)
// and a boolean defining if the projects volume is generally necessary for the workspace (covering cases 1 and 2)
func processProjectsVolume(workspace *dw.DevWorkspaceTemplateSpec) (projectsComponent *dw.Component, needed bool) {
	// If any container has mountSources == true, we need projects
	needed = containerlib.AnyMountSources(workspace.Components)
	for _, component := range workspace.Components {
		if component.Volume != nil && component.Name == devfileConstants.ProjectsVolumeName {
			projectsComponent = &component
			// Add projects volume if it's explicitly defined, even if it's not used anywhere
			needed = true
		}
	}
	return
}

// checkForAlternatePVC checks the current namespace for existing PVCs that may be used for workspace storage. If
// such a PVC is found, its name is returned and should be used in place of the configured common PVC. If no suitable
// PVC is found, the returned PVC name is an empty string and a nil error is returned. If an error occurs during the lookup,
// then an empty string is returned as well as the error.
// Currently, the only alternate PVC that can be used is named `claim-che-workspace`.
func checkForAlternatePVC(namespace string, api sync.ClusterAPI) (exists bool, name string, err error) {
	existingPVC := &corev1.PersistentVolumeClaim{}
	namespacedName := types.NamespacedName{Name: constants.CheCommonPVCName, Namespace: namespace}
	err = api.Client.Get(api.Ctx, namespacedName, existingPVC)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return false, "", nil
		}
		return false, "", err
	}
	return true, existingPVC.Name, nil
}

// getSharedPVCWorkspaceCount returns the total number of workspaces which are using a shared PVC
// (i.e the workspaces storage-class attribute is set to "common", "async", or unset which defaults to "common")
// Note that workspaces that are have been deleted (i.e. have a deletion timestamp) are not counted.
func getSharedPVCWorkspaceCount(namespace string, api sync.ClusterAPI) (total int, err error) {
	workspaces := &dw.DevWorkspaceList{}
	err = api.Client.List(api.Ctx, workspaces, &client.ListOptions{Namespace: namespace})
	if err != nil {
		return 0, err
	}
	for _, workspace := range workspaces.Items {
		if workspace.DeletionTimestamp != nil {
			// Ignore terminating workspaces
			continue
		}
		storageClass := workspace.Spec.Template.Attributes.GetString(constants.DevWorkspaceStorageTypeAttribute, nil)
		// Note, if the storageClass attribute isn't set (ie. storageClass == ""), then the storage class being used is "common"
		if storageClass == constants.AsyncStorageClassType || storageClass == constants.CommonStorageClassType || storageClass == constants.PerUserStorageClassType || storageClass == "" {
			total++
		}
	}
	return total, nil
}

func checkPVCTerminating(name, namespace string, api sync.ClusterAPI) (bool, error) {
	if name == "" {
		// Should not happen
		return false, fmt.Errorf("attempted to read deletion status of PVC with empty name")
	}
	pvc := &corev1.PersistentVolumeClaim{}
	namespacedName := types.NamespacedName{Name: name, Namespace: namespace}
	err := api.Client.Get(api.Ctx, namespacedName, pvc)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return pvc.DeletionTimestamp != nil, nil
}
