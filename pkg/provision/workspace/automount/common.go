//
// Copyright (c) 2019-2021 Red Hat, Inc.
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
package automount

import (
	"fmt"

	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

type FatalError struct {
	Err error
}

func (e *FatalError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return ""
}

func (e *FatalError) Unwrap() error {
	return e.Err
}

func GetAutoMountResources(client k8sclient.Client, namespace string) ([]v1alpha1.PodAdditions, []corev1.EnvFromSource, error) {
	gitCMPodAdditions, err := getDevWorkspaceGitConfig(client, namespace)
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
