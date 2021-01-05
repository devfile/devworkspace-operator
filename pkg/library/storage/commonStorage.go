//
// Copyright (c) 2019-2020 Red Hat, Inc.
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

	corev1 "k8s.io/api/core/v1"

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config"
)

func RewriteContainerVolumeMounts(workspaceId string, podAdditions *v1alpha1.PodAdditions, devfile devworkspace.DevWorkspaceTemplateSpec) error {
	devfileVolumes := map[string]devworkspace.VolumeComponent{}
	for _, component := range devfile.Components {
		if component.Volume != nil {
			if _, exists := devfileVolumes[component.Name]; exists {
				return fmt.Errorf("volume component %s is defined multiple times", component.Name)
			}
			devfileVolumes[component.Name] = *component.Volume
		}
	}
	// TODO: Support more than the common PVC strategy here (storage provisioner interface?)
	// TODO: What should we do when a volume isn't explicitly defined?
	commonPVCName := config.ControllerCfg.GetWorkspacePVCName()
	for cIdx, container := range podAdditions.Containers {
		for vmIdx, vm := range container.VolumeMounts {
			if _, ok := devfileVolumes[vm.Name]; !ok {
				return fmt.Errorf("container %s references undefined volume %s", container.Name, vm.Name)
			}
			podAdditions.Containers[cIdx].VolumeMounts[vmIdx].SubPath = fmt.Sprintf("%s/%s", workspaceId, vm.Name)
			podAdditions.Containers[cIdx].VolumeMounts[vmIdx].Name = commonPVCName
		}
	}
	for cIdx, container := range podAdditions.InitContainers {
		for vmIdx, vm := range container.VolumeMounts {
			if _, ok := devfileVolumes[vm.Name]; !ok {
				return fmt.Errorf("container %s references undefined volume %s", container.Name, vm.Name)
			}
			podAdditions.InitContainers[cIdx].VolumeMounts[vmIdx].SubPath = fmt.Sprintf("%s/%s", workspaceId, vm.Name)
			podAdditions.InitContainers[cIdx].VolumeMounts[vmIdx].Name = commonPVCName
		}
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
