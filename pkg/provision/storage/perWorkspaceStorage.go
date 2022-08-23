//
// Copyright (c) 2019-2022 Red Hat, Inc.
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

func (p *PerWorkspaceStorageProvisioner) ProvisionStorage(podAdditions *v1alpha1.PodAdditions, workspaceWithConfig *common.DevWorkspaceWithConfig, clusterAPI sync.ClusterAPI) error {
	// Add ephemeral volumes
	if err := addEphemeralVolumesFromWorkspace(&workspaceWithConfig.DevWorkspace, podAdditions); err != nil {
		return err
	}

	// If persistent storage is not needed, we're done
	if !needsStorage(&workspaceWithConfig.Spec.Template) {
		return nil
	}

	// Get perWorkspace PVC spec and sync it with cluster
	perWorkspacePVC, err := syncPerWorkspacePVC(workspaceWithConfig, clusterAPI)
	if err != nil {
		return err
	}
	pvcName := perWorkspacePVC.Name

	// If PVC is being deleted, we need to fail workspace startup as a running pod will block deletion.
	if perWorkspacePVC.DeletionTimestamp != nil {
		return &ProvisioningError{
			Message: "DevWorkspace PVC is being deleted",
		}
	}

	// Rewrite container volume mounts
	if err := p.rewriteContainerVolumeMounts(workspaceWithConfig.Status.DevWorkspaceId, pvcName, podAdditions, &workspaceWithConfig.Spec.Template); err != nil {
		return &ProvisioningError{
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

func syncPerWorkspacePVC(workspaceWithConfig *common.DevWorkspaceWithConfig, clusterAPI sync.ClusterAPI) (*corev1.PersistentVolumeClaim, error) {
	namespacedConfig, err := nsconfig.ReadNamespacedConfig(workspaceWithConfig.Namespace, clusterAPI)
	if err != nil {
		return nil, fmt.Errorf("failed to read namespace-specific configuration: %w", err)
	}
	// TODO: Determine the storage size that is needed by iterating through workspace volumes,
	// adding the sizes specified and figuring out overrides/defaults
	pvcSize := *workspaceWithConfig.Config.Workspace.DefaultStorageSize.PerWorkspace
	if namespacedConfig != nil && namespacedConfig.PerWorkspacePVCSize != "" {
		pvcSize, err = resource.ParseQuantity(namespacedConfig.PerWorkspacePVCSize)
		if err != nil {
			return nil, err
		}
	}

	pvc, err := getPVCSpec(common.PerWorkspacePVCName(workspaceWithConfig.Status.DevWorkspaceId), workspaceWithConfig.Namespace, pvcSize, workspaceWithConfig.Config)
	if err != nil {
		return nil, err
	}

	if err := controllerutil.SetControllerReference(&workspaceWithConfig.DevWorkspace, pvc, clusterAPI.Scheme); err != nil {
		return nil, err
	}

	currObject, err := sync.SyncObjectWithCluster(pvc, clusterAPI)
	switch t := err.(type) {
	case nil:
		break
	case *sync.NotInSyncError:
		return nil, &NotReadyError{
			Message: fmt.Sprintf("Updated %s PVC on cluster", pvc.Name),
		}
	case *sync.UnrecoverableSyncError:
		return nil, &ProvisioningError{
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
