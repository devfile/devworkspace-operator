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

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
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

type automountResources struct {
	Volumes       []corev1.Volume
	VolumeMounts  []corev1.VolumeMount
	EnvFromSource []corev1.EnvFromSource
}

func ProvisionAutoMountResourcesInto(podAdditions []v1alpha1.PodAdditions, api sync.ClusterAPI, namespace string) error {
	return nil
}

func getAutomountResources(api sync.ClusterAPI, namespace string) (*automountResources, error) {
	gitCMAutoMountResources, err := provisionGitConfiguration(api, namespace)
	if err != nil {
		return nil, err
	}

	cmAutoMountResources, err := getDevWorkspaceConfigmaps(namespace, api)
	if err != nil {
		return nil, err
	}

	secretAutoMountResources, err := getDevWorkspaceSecrets(namespace, api)
	if err != nil {
		return nil, err
	}

	pvcAutoMountResources, err := getAutoMountPVCs(namespace, api)
	if err != nil {
		return nil, err
	}

	return mergeAutomountResources(gitCMAutoMountResources, cmAutoMountResources, secretAutoMountResources, pvcAutoMountResources), nil
}

func checkAutomountVolumesForCollision(podAdditions *v1alpha1.PodAdditions, automount *automountResources) error {
	// Get a map of automounted volume names to volume structs
	automountVolumeNames := map[string]corev1.Volume{}
	for _, volume := range automount.Volumes {
		automountVolumeNames[volume.Name] = volume
	}

	// Check that workspace volumes do not conflict with automounted volumes
	for _, volume := range podAdditions.Volumes {
		if conflict, exists := automountVolumeNames[volume.Name]; exists {
			return &FatalError{fmt.Errorf("DevWorkspace volume '%s' conflicts with automounted volume from %s",
				volume.Name, formatVolumeDescription(conflict))}
		}
	}

	// Check that automounted mountPaths do not collide
	automountVolumeMountsByMountPath := map[string]corev1.VolumeMount{}
	for _, vm := range automount.VolumeMounts {
		if conflict, exists := automountVolumeMountsByMountPath[vm.MountPath]; exists {
			return &FatalError{fmt.Errorf("auto-mounted volumes from %s and %s have the same mount path",
				getVolumeDescriptionFromVolumeMount(vm, automount.Volumes), getVolumeDescriptionFromVolumeMount(conflict, automount.Volumes))}
		}
		automountVolumeMountsByMountPath[vm.MountPath] = vm
	}

	// Check that automounted volume mountPaths do not conflict with existing mountPaths in any container
	for _, container := range podAdditions.Containers {
		for _, vm := range container.VolumeMounts {
			if conflict, exists := automountVolumeMountsByMountPath[vm.MountPath]; exists {
				return &FatalError{fmt.Errorf("DevWorkspace volume %s in container %s has same mountpath as auto-mounted volume from %s",
					getVolumeDescriptionFromVolumeMount(vm, podAdditions.Volumes), container.Name, getVolumeDescriptionFromVolumeMount(conflict, automount.Volumes))}
			}
		}
	}
	return nil
}

func mergeAutomountResources(resources ...*automountResources) *automountResources {
	result := &automountResources{}
	for _, resource := range resources {
		if resource == nil {
			continue
		}
		result.Volumes = append(result.Volumes, resource.Volumes...)
		result.VolumeMounts = append(result.VolumeMounts, resource.VolumeMounts...)
		result.EnvFromSource = append(result.EnvFromSource, resource.EnvFromSource...)
	}
	return result
}

// getVolumeDescriptionFromAutoVolumeMount takes a volumeMount and list of volumes and returns a formatted description of
// the underlying volume (i.e. "secret <secretName>" or "configmap <configmapName>" as defined by formatVolumeDescription).
// If the volume referred to by the volumeMount does not exist in the volumes slice, the volumeMount's name is returned
func getVolumeDescriptionFromVolumeMount(vm corev1.VolumeMount, volumes []corev1.Volume) string {
	for _, volume := range volumes {
		if volume.Name == vm.Name {
			return formatVolumeDescription(volume)
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
