//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package storage

import (
	"errors"
	"fmt"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/workspace/provision"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	devfileConstants "github.com/devfile/devworkspace-operator/pkg/library/constants"
	containerlib "github.com/devfile/devworkspace-operator/pkg/library/container"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getCommonPVCSpec(namespace string) (*corev1.PersistentVolumeClaim, error) {
	pvcStorageQuantity, err := resource.ParseQuantity(constants.PVCStorageSize)
	if err != nil {
		return nil, err
	}

	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ControllerCfg.GetWorkspacePVCName(),
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": pvcStorageQuantity,
				},
			},
			StorageClassName: config.ControllerCfg.GetPVCStorageClassName(),
		},
	}, nil
}

// needsStorage returns true if storage will need to be provisioned for the current workspace. Note that ephemeral volumes
// do not need to provision storage
func needsStorage(workspace *dw.DevWorkspaceTemplateSpec) bool {
	projectsVolumeIsEphemeral := false
	for _, component := range workspace.Components {
		if component.Volume != nil {
			// If any non-ephemeral volumes are defined, we need to mount storage
			if !component.Volume.Ephemeral {
				return true
			}
			if component.Name == devfileConstants.ProjectsVolumeName {
				projectsVolumeIsEphemeral = component.Volume.Ephemeral
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

func syncCommonPVC(namespace string, clusterAPI provision.ClusterAPI) (*corev1.PersistentVolumeClaim, error) {
	pvc, err := getCommonPVCSpec(namespace)
	if err != nil {
		return nil, err
	}
	currObject, requeue, err := provision.SyncObject(pvc, clusterAPI.Client, clusterAPI.Logger, false)
	if err != nil {
		return nil, err
	}
	if requeue {
		return nil, &NotReadyError{
			Message: "Updated common PVC on cluster",
		}
	}
	currPVC, ok := currObject.(*corev1.PersistentVolumeClaim)
	if !ok {
		return nil, errors.New("tried to sync PVC to cluster but did not get a PVC back")
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
func addEphemeralVolumesFromWorkspace(workspace *dw.DevWorkspace, podAdditions *v1alpha1.PodAdditions) error {
	_, ephemeralVolumes, projectsVolume := getWorkspaceVolumes(workspace)
	_, err := addEphemeralVolumesToPodAdditions(podAdditions, ephemeralVolumes)
	if err != nil {
		return &ProvisioningError{Message: "Failed to add ephemeral volumes to workspace", Err: err}
	}
	if projectsVolume != nil && projectsVolume.Volume.Ephemeral {
		if _, err := addEphemeralVolumesToPodAdditions(podAdditions, []dw.Component{*projectsVolume}); err != nil {
			return &ProvisioningError{Message: "Failed to add projects volume to workspace", Err: err}
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
func getWorkspaceVolumes(workspace *dw.DevWorkspace) (persistent, ephemeral []dw.Component, projects *dw.Component) {
	for _, component := range workspace.Spec.Template.Components {
		if component.Volume == nil {
			continue
		}
		if component.Name == devfileConstants.ProjectsVolumeName {
			projects = &component
			continue
		}
		if component.Volume.Ephemeral {
			ephemeral = append(ephemeral, component)
		} else {
			persistent = append(persistent, component)
		}
	}
	return persistent, ephemeral, projects
}

// processProjectsVolume handles the special case of the projects volume, for which there are four possibilities:
// 1. The projects volume is not needed for the workspace (no component has mountSources: true)
// 2. The projects volume is needed but not defined in the devfile. This is the usual case, as the projects volume
//    is implicitly defined by mountSources
// 3. The projects volume is explicitly defined in the workspace, as a regular volume.
// 4. The projects volume is explicitly defined in the workspace as an ephemeral volume.
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
