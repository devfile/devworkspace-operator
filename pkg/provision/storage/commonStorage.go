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
	"fmt"
	"time"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"

	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	devfileConstants "github.com/devfile/devworkspace-operator/pkg/library/constants"
)

// The CommonStorageProvisioner provisions one PVC per namespace and configures all volumes in a workspace
// to mount on subpaths within that PVC. Workspace storage is mounted prefixed with the workspace ID.
type CommonStorageProvisioner struct{}

var _ Provisioner = (*CommonStorageProvisioner)(nil)

func (*CommonStorageProvisioner) NeedsStorage(workspace *dw.DevWorkspaceTemplateSpec) bool {
	return needsStorage(workspace)
}

func (p *CommonStorageProvisioner) ProvisionStorage(podAdditions *v1alpha1.PodAdditions, workspace *common.DevWorkspaceWithConfig, clusterAPI sync.ClusterAPI) error {
	// Add ephemeral volumes
	if err := addEphemeralVolumesFromWorkspace(workspace, podAdditions); err != nil {
		return err
	}

	// If persistent storage is not needed, we're done
	if !p.NeedsStorage(&workspace.Spec.Template) {
		return nil
	}

	usingAlternatePVC, pvcName, err := checkForAlternatePVC(workspace.Namespace, clusterAPI)
	if err != nil {
		return err
	}

	if pvcName == "" {
		pvcName = workspace.Config.Workspace.PVCName
	}
	pvcTerminating, err := checkPVCTerminating(pvcName, workspace.Namespace, clusterAPI)
	if err != nil {
		return err
	} else if pvcTerminating {
		return &dwerrors.RetryError{
			Message:      "Shared PVC is in terminating state",
			RequeueAfter: 2 * time.Second,
		}
	}

	if !usingAlternatePVC {
		commonPVC, err := syncCommonPVC(workspace.Namespace, workspace.Config, clusterAPI)
		if err != nil {
			return err
		}
		pvcName = commonPVC.Name
	}

	if err := p.rewriteContainerVolumeMounts(workspace.Status.DevWorkspaceId, pvcName, podAdditions, &workspace.Spec.Template); err != nil {
		return &dwerrors.FailError{
			Err:     err,
			Message: "Could not rewrite container volume mounts",
		}
	}

	return nil
}

func (p *CommonStorageProvisioner) CleanupWorkspaceStorage(workspace *common.DevWorkspaceWithConfig, clusterAPI sync.ClusterAPI) error {
	totalWorkspaces, err := getSharedPVCWorkspaceCount(workspace.Namespace, clusterAPI)
	if err != nil {
		return err
	}

	// If the number of common + async workspaces that exist (started or stopped) is zero,
	// delete common PVC instead of running cleanup job
	if totalWorkspaces > 0 {
		return runCommonPVCCleanupJob(workspace, clusterAPI)
	} else {
		sharedPVC := &corev1.PersistentVolumeClaim{}
		namespacedName := types.NamespacedName{Name: workspace.Config.Workspace.PVCName, Namespace: workspace.Namespace}
		err := clusterAPI.Client.Get(clusterAPI.Ctx, namespacedName, sharedPVC)

		if err != nil {
			if k8sErrors.IsNotFound(err) {
				return nil
			}
			return err
		}

		err = clusterAPI.Client.Delete(clusterAPI.Ctx, sharedPVC)
		if err != nil && !k8sErrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

// rewriteContainerVolumeMounts rewrites the VolumeMounts in a set of PodAdditions according to the 'common' PVC strategy
// (i.e. all volume mounts are subpaths into a common PVC used by all workspaces in the namespace).
//
// Also adds appropriate k8s Volumes to PodAdditions to accomodate the rewritten VolumeMounts.
func (p *CommonStorageProvisioner) rewriteContainerVolumeMounts(workspaceId, pvcName string, podAdditions *v1alpha1.PodAdditions, workspace *dw.DevWorkspaceTemplateSpec) error {
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
					containers[cIdx].VolumeMounts[vmIdx].SubPath = fmt.Sprintf("%s/%s", workspaceId, vm.Name)
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
