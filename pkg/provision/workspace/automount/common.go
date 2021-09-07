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

package automount

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func GetAutoMountResources(devworkspace *dw.DevWorkspace, client k8sclient.Client, scheme *runtime.Scheme) ([]v1alpha1.PodAdditions, []corev1.EnvFromSource, error) {
	namespace := devworkspace.GetNamespace()
	gitCMPodAdditions, err := getDevWorkspaceGitConfig(devworkspace, client, scheme)
	if err != nil {
		return nil, nil, err
	}

	cmPodAdditions, cmEnvAdditions, err := getDevWorkspaceConfigmaps(namespace, client)
	if err != nil {
		return nil, nil, err
	}
	secretPodAdditions, secretEnvAdditions, err := getDevWorkspaceSecrets(namespace, client)
	if err != nil {
		return nil, nil, err
	}
	pvcPodAdditions, err := getAutoMountPVCs(namespace, client)
	if err != nil {
		return nil, nil, err
	}

	var allPodAdditions []v1alpha1.PodAdditions
	if gitCMPodAdditions != nil {
		allPodAdditions = append(allPodAdditions, *gitCMPodAdditions)
	}
	if cmPodAdditions != nil {
		allPodAdditions = append(allPodAdditions, *cmPodAdditions)
	}
	if secretPodAdditions != nil {
		allPodAdditions = append(allPodAdditions, *secretPodAdditions)
	}
	if pvcPodAdditions != nil {
		allPodAdditions = append(allPodAdditions, *pvcPodAdditions)
	}

	return allPodAdditions, append(cmEnvAdditions, secretEnvAdditions...), nil
}

func CheckAutoMountVolumesForCollision(base, automount []v1alpha1.PodAdditions) error {
	// Get a map of automounted volume names to volume structs
	automountVolumeNames := map[string]corev1.Volume{}
	for _, podAddition := range automount {
		for _, volume := range podAddition.Volumes {
			automountVolumeNames[volume.Name] = volume
		}
	}

	// Check that workspace volumes do not conflict with automounted volumes
	for _, podAddition := range base {
		for _, volume := range podAddition.Volumes {
			if conflict, exists := automountVolumeNames[volume.Name]; exists {
				return fmt.Errorf("DevWorkspace volume '%s' conflicts with automounted volume from %s",
					volume.Name, formatVolumeDescription(conflict))
			}
		}
	}

	// Check that automounted mountPaths do not collide
	automountVolumeMountsByMountPath := map[string]corev1.VolumeMount{}
	for _, podAddition := range automount {
		for _, vm := range podAddition.VolumeMounts {
			if conflict, exists := automountVolumeMountsByMountPath[vm.MountPath]; exists {
				return fmt.Errorf("auto-mounted volumes from %s and %s have the same mount path",
					getVolumeDescriptionFromVolumeMount(vm, automount), getVolumeDescriptionFromVolumeMount(conflict, automount))
			}
			automountVolumeMountsByMountPath[vm.MountPath] = vm
		}
	}

	// Check that automounted volume mountPaths do not conflict with existing mountPaths in any container
	for _, podAddition := range base {
		for _, container := range podAddition.Containers {
			for _, vm := range container.VolumeMounts {
				if conflict, exists := automountVolumeMountsByMountPath[vm.MountPath]; exists {
					return fmt.Errorf("DevWorkspace volume %s in container %s has same mountpath as auto-mounted volume from %s",
						getVolumeDescriptionFromVolumeMount(vm, base), container.Name, getVolumeDescriptionFromVolumeMount(conflict, automount))
				}
			}
		}
	}
	return nil
}

// getVolumeDescriptionFromVolumeMount takes a volumeMount and returns a formatted description of the underlying volume
// (i.e. "secret <secretName>" or "configmap <configmapName>" as defined by formatVolumeDescription) the provided slice
// of podAdditions. If a match cannot be found, the volumeMount name is returned.
func getVolumeDescriptionFromVolumeMount(vm corev1.VolumeMount, podAdditions []v1alpha1.PodAdditions) string {
	for _, podAddition := range podAdditions {
		for _, volume := range podAddition.Volumes {
			if volume.Name == vm.Name {
				return formatVolumeDescription(volume)
			}
		}
	}
	return vm.Name
}

// formatVolumeDescription formats a given volume as either "configmap '<configmap-name>'" or "secret '<secret-name>'",
// depending on whether the volume refers to a configmap or secret. If the volume is neither a secret nor configmap,
// returns the name of the volume itself.
func formatVolumeDescription(vol corev1.Volume) string {
	if vol.Secret != nil {
		return fmt.Sprintf("secret '%s'", vol.Secret.SecretName)
	} else if vol.ConfigMap != nil {
		return fmt.Sprintf("configmap '%s'", vol.ConfigMap.Name)
	}
	return fmt.Sprintf("'%s'", vol.Name)
}
