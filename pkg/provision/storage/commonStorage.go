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
	"fmt"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"

	corev1 "k8s.io/api/core/v1"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	devfileConstants "github.com/devfile/devworkspace-operator/pkg/library/constants"
	wsprovision "github.com/devfile/devworkspace-operator/pkg/provision/workspace"
)

// The CommonStorageProvisioner provisions one PVC per namespace and configures all volumes in a workspace
// to mount on subpaths within that PVC. Workspace storage is mounted prefixed with the workspace ID.
type CommonStorageProvisioner struct{}

var _ Provisioner = (*CommonStorageProvisioner)(nil)

func (*CommonStorageProvisioner) NeedsStorage(workspace *dw.DevWorkspaceTemplateSpec) bool {
	return needsStorage(workspace)
}

func (p *CommonStorageProvisioner) ProvisionStorage(podAdditions *v1alpha1.PodAdditions, workspace *dw.DevWorkspace, clusterAPI wsprovision.ClusterAPI) error {
	// Add ephemeral volumes
	if err := addEphemeralVolumesFromWorkspace(workspace, podAdditions); err != nil {
		return err
	}

	// If persistent storage is not needed, we're done
	if !p.NeedsStorage(&workspace.Spec.Template) {
		return nil
	}

	if err := p.rewriteContainerVolumeMounts(workspace.Status.DevWorkspaceId, podAdditions, &workspace.Spec.Template); err != nil {
		return &ProvisioningError{
			Err:     err,
			Message: "Could not rewrite container volume mounts",
		}
	}

	if _, err := syncCommonPVC(workspace.Namespace, clusterAPI); err != nil {
		return err
	}
	return nil
}

func (*CommonStorageProvisioner) CleanupWorkspaceStorage(workspace *dw.DevWorkspace, clusterAPI wsprovision.ClusterAPI) error {
	return runCommonPVCCleanupJob(workspace, clusterAPI)
}

// rewriteContainerVolumeMounts rewrites the VolumeMounts in a set of PodAdditions according to the 'common' PVC strategy
// (i.e. all volume mounts are subpaths into a common PVC used by all workspaces in the namespace).
//
// Also adds appropriate k8s Volumes to PodAdditions to accomodate the rewritten VolumeMounts.
func (p *CommonStorageProvisioner) rewriteContainerVolumeMounts(workspaceId string, podAdditions *v1alpha1.PodAdditions, workspace *dw.DevWorkspaceTemplateSpec) error {
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

	// Add implicit projects volume to support mountSources, if needed
	if _, exists := devfileVolumes[devfileConstants.ProjectsVolumeName]; !exists {
		projectsVolume := dw.VolumeComponent{}
		projectsVolume.Size = constants.PVCStorageSize
		devfileVolumes[devfileConstants.ProjectsVolumeName] = projectsVolume
	}

	// TODO: What should we do when a volume isn't explicitly defined?
	commonPVCName := config.ControllerCfg.GetWorkspacePVCName()
	rewriteVolumeMounts := func(containers []corev1.Container) error {
		for cIdx, container := range containers {
			for vmIdx, vm := range container.VolumeMounts {
				volume, ok := devfileVolumes[vm.Name]
				if !ok {
					return fmt.Errorf("container '%s' references undefined volume '%s'", container.Name, vm.Name)
				}
				if !volume.Ephemeral {
					containers[cIdx].VolumeMounts[vmIdx].SubPath = fmt.Sprintf("%s/%s", workspaceId, vm.Name)
					containers[cIdx].VolumeMounts[vmIdx].Name = commonPVCName
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
		Name: commonPVCName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: commonPVCName,
			},
		},
	})

	return nil
}
