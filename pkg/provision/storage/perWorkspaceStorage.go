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
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	devfileConstants "github.com/devfile/devworkspace-operator/pkg/library/constants"
	nsconfig "github.com/devfile/devworkspace-operator/pkg/provision/config"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// The PerWorkspaceStorageProvisioner provisions one PVC per workspace and configures all volumes in the workspace
// to mount on subpaths within that PVC.
type PerWorkspaceStorageProvisioner struct{}

var _ Provisioner = (*PerWorkspaceStorageProvisioner)(nil)

// This function is used to determine whether a finalizer should be applied to the DevWorkspace.
// Since the per-workspace PVC are cleaned up/deleted via Owner References when the workspace is deleted,
// no finalizer needs to be applied
func (*PerWorkspaceStorageProvisioner) NeedsStorage(workspace *dw.DevWorkspaceTemplateSpec) bool {
	return false
}

func (p *PerWorkspaceStorageProvisioner) ProvisionStorage(podAdditions *v1alpha1.PodAdditions, workspace *common.DevWorkspaceWithConfig, clusterAPI sync.ClusterAPI) error {
	// Add ephemeral volumes
	if err := addEphemeralVolumesFromWorkspace(workspace, podAdditions); err != nil {
		return err
	}

	// If persistent storage is not needed, we're done
	if !needsStorage(&workspace.Spec.Template) {
		return nil
	}

	// Get perWorkspace PVC spec and sync it with cluster
	perWorkspacePVC, err := syncPerWorkspacePVC(workspace, clusterAPI)
	if err != nil {
		return err
	}
	pvcName := perWorkspacePVC.Name

	// If PVC is being deleted, we need to fail workspace startup as a running pod will block deletion.
	if perWorkspacePVC.DeletionTimestamp != nil {
		return &dwerrors.FailError{
			Message: "DevWorkspace PVC is being deleted",
		}
	}

	// Rewrite container volume mounts
	if err := p.rewriteContainerVolumeMounts(workspace.Status.DevWorkspaceId, pvcName, podAdditions, &workspace.Spec.Template); err != nil {
		return &dwerrors.FailError{
			Err:     err,
			Message: "Could not rewrite container volume mounts",
		}
	}

	return nil
}

// We rely on Kubernetes to use the owner reference to automatically delete the PVC once the workspace is set for deletion.
func (*PerWorkspaceStorageProvisioner) CleanupWorkspaceStorage(workspace *common.DevWorkspaceWithConfig, clusterAPI sync.ClusterAPI) error {
	return nil
}

// rewriteContainerVolumeMounts rewrites the VolumeMounts in a set of PodAdditions according to the 'per-workspace' PVC strategy
// (i.e. all volume mounts are subpaths into a PVC used by a single workspace in the namespace).
//
// Also adds appropriate k8s Volumes to PodAdditions to accomodate the rewritten VolumeMounts.
func (p *PerWorkspaceStorageProvisioner) rewriteContainerVolumeMounts(workspaceId, pvcName string, podAdditions *v1alpha1.PodAdditions, workspace *dw.DevWorkspaceTemplateSpec) error {
	devfileVolumes := map[string]dw.VolumeComponent{}

	// Construct map of volume name -> volume Component
	for _, component := range workspace.Components {
		if component.Volume != nil {
			if _, exists := devfileVolumes[component.Name]; exists {
				return fmt.Errorf("volume component '%s' is defined multiple times", component.Name)
			}
			devfileVolumes[component.Name] = *component.Volume
		}
	}

	// Containers in podAdditions may reference e.g. automounted volumes in their volumeMounts, and this is not an error
	additionalVolumes := map[string]bool{}
	for _, additionalVolume := range podAdditions.Volumes {
		additionalVolumes[additionalVolume.Name] = true
	}

	// Add implicit projects volume to support mountSources, if needed
	if _, exists := devfileVolumes[devfileConstants.ProjectsVolumeName]; !exists {
		projectsVolume := dw.VolumeComponent{}
		projectsVolume.Size = constants.PVCStorageSize
		devfileVolumes[devfileConstants.ProjectsVolumeName] = projectsVolume
	}

	// TODO: What should we do when a volume isn't explicitly defined?
	rewriteVolumeMounts := func(containers []corev1.Container) error {
		for cIdx, container := range containers {
			for vmIdx, vm := range container.VolumeMounts {
				volume, ok := devfileVolumes[vm.Name]
				if !ok {
					// Volume is defined outside of the devfile
					if additionalVolumes[vm.Name] {
						continue
					}
					// Should never happen as flattened Devfile is validated.
					return fmt.Errorf("container '%s' references undefined volume '%s'", container.Name, vm.Name)
				}
				if !isEphemeral(&volume) {
					containers[cIdx].VolumeMounts[vmIdx].SubPath = vm.Name
					containers[cIdx].VolumeMounts[vmIdx].Name = pvcName
				}
			}
		}
		return nil
	}
	if err := rewriteVolumeMounts(podAdditions.Containers); err != nil {
		return err
	}
	if err := rewriteVolumeMounts(podAdditions.InitContainers); err != nil {
		return err
	}

	podAdditions.Volumes = append(podAdditions.Volumes, corev1.Volume{
		Name: pvcName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
			},
		},
	})

	return nil
}

func getPVCSize(workspace *common.DevWorkspaceWithConfig, namespacedConfig *nsconfig.NamespacedConfig) (*resource.Quantity, error) {
	defaultPVCSize := *workspace.Config.Workspace.DefaultStorageSize.PerWorkspace

	// Calculate required PVC size based on workspace volumes
	allVolumeSizesDefined := true
	requiredPVCSize := resource.NewQuantity(0, resource.BinarySI)
	for _, component := range workspace.Spec.Template.Components {
		if component.Volume != nil {
			if isEphemeral(component.Volume) {
				continue
			}

			if component.Volume.Size == "" {
				allVolumeSizesDefined = false
				continue
			}

			volumeSize, err := resource.ParseQuantity(component.Volume.Size)
			if err != nil {
				return nil, err
			}
			requiredPVCSize.Add(volumeSize)
		}
	}

	// Use the calculated PVC size if it's greater than default PVC size
	if allVolumeSizesDefined || requiredPVCSize.Cmp(defaultPVCSize) == 1 {
		return requiredPVCSize, nil
	}

	if namespacedConfig != nil && namespacedConfig.PerWorkspacePVCSize != "" {
		pvcSize, err := resource.ParseQuantity(namespacedConfig.PerWorkspacePVCSize)
		if err != nil {
			return nil, err
		}
		return &pvcSize, nil
	}

	return &defaultPVCSize, nil
}

func syncPerWorkspacePVC(workspace *common.DevWorkspaceWithConfig, clusterAPI sync.ClusterAPI) (*corev1.PersistentVolumeClaim, error) {
	namespacedConfig, err := nsconfig.ReadNamespacedConfig(workspace.Namespace, clusterAPI)
	if err != nil {
		return nil, fmt.Errorf("failed to read namespace-specific configuration: %w", err)
	}

	pvcSize, err := getPVCSize(workspace, namespacedConfig)
	if err != nil {
		return nil, err
	}

	storageClass := workspace.Config.Workspace.StorageClassName
	pvc, err := getPVCSpec(common.PerWorkspacePVCName(workspace.Status.DevWorkspaceId), workspace.Namespace, storageClass, *pvcSize)
	if err != nil {
		return nil, err
	}
	if pvc.Labels == nil {
		pvc.Labels = map[string]string{}
	}
	pvc.Labels[constants.DevWorkspaceIDLabel] = workspace.Status.DevWorkspaceId

	if err := controllerutil.SetControllerReference(workspace.DevWorkspace, pvc, clusterAPI.Scheme); err != nil {
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
		return nil, errors.New("tried to sync per-workspace PVC to cluster but did not get a PVC back")
	}

	return currPVC, nil
}
