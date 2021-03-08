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

// Package storage contains library functions for provisioning volumes and volumeMounts in containers according to the
// volume components in a devfile. These functions also handle mounting project sources to containers that require it.
//
// TODO:
// - Add functionality for generating PVCs with the appropriate size based on size requests in the devfile
// - Devfile API spec is unclear on how mountSources should be handled -- mountPath is assumed to be /projects
//   and volume name is assumed to be "projects"
//   see issues:
//     - https://github.com/devfile/api/issues/290
//     - https://github.com/devfile/api/issues/291
package storage

import (
	"fmt"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	devfileConstants "github.com/devfile/devworkspace-operator/pkg/library/constants"
	containerlib "github.com/devfile/devworkspace-operator/pkg/library/container"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	devworkspace "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
)

// RewriteContainerVolumeMounts rewrites the VolumeMounts in a set of PodAdditions according to the 'common' PVC strategy
// (i.e. all volume mounts are subpaths into a common PVC used by all workspaces in the namespace).
//
// Also adds appropriate k8s Volumes to PodAdditions to accomodate the rewritten VolumeMounts.
func RewriteContainerVolumeMounts(workspaceId string, podAdditions *v1alpha1.PodAdditions, workspace devworkspace.DevWorkspaceTemplateSpec) error {
	devfileVolumes := map[string]devworkspace.VolumeComponent{}
	var ephemeralVolumes []devworkspace.Component

	for _, component := range workspace.Components {
		if component.Volume != nil {
			if _, exists := devfileVolumes[component.Name]; exists {
				return fmt.Errorf("volume component '%s' is defined multiple times", component.Name)
			}
			devfileVolumes[component.Name] = *component.Volume
			if component.Volume.Ephemeral {
				ephemeralVolumes = append(ephemeralVolumes, component)
			}
		}
	}
	if _, exists := devfileVolumes[devfileConstants.ProjectsVolumeName]; !exists {
		// Add implicit projects volume to support mountSources
		projectsVolume := devworkspace.VolumeComponent{}
		projectsVolume.Size = constants.PVCStorageSize
		devfileVolumes[devfileConstants.ProjectsVolumeName] = projectsVolume
	}

	for _, component := range ephemeralVolumes {
		vol := corev1.Volume{
			Name: component.Name,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
		if component.Volume.Size != "" {
			sizeResource, err := resource.ParseQuantity(component.Volume.Size)
			if err != nil {
				return fmt.Errorf("failed to parse size for Volume %s: %w", component.Name, err)
			}
			vol.EmptyDir.SizeLimit = &sizeResource
		}
		podAdditions.Volumes = append(podAdditions.Volumes, vol)
	}

	if NeedsStorage(workspace) {
		// TODO: Support more than the common PVC strategy here (storage provisioner interface?)
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
	}

	return nil
}

// NeedsStorage returns true if storage will need to be provisioned for the current workspace. Note that ephemeral volumes
// do not need to provision storage
func NeedsStorage(workspace devworkspace.DevWorkspaceTemplateSpec) bool {
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
