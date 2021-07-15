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

package provision

import (
	"context"
	"fmt"
	"path"

	corev1 "k8s.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

func getAutoMountResources(namespace string, client runtimeClient.Client) ([]v1alpha1.PodAdditions, []corev1.EnvFromSource, error) {
	cmPodAdditions, cmEnvAdditions, err := getDevWorkspaceConfigmaps(namespace, client)
	if err != nil {
		return nil, nil, err
	}
	secretPodAdditions, secretEnvAdditions, err := getDevWorkspaceSecrets(namespace, client)
	if err != nil {
		return nil, nil, err
	}

	var allPodAdditions []v1alpha1.PodAdditions
	if cmPodAdditions != nil {
		allPodAdditions = append(allPodAdditions, *cmPodAdditions)
	}
	if secretPodAdditions != nil {
		allPodAdditions = append(allPodAdditions, *secretPodAdditions)
	}

	return allPodAdditions, append(cmEnvAdditions, secretEnvAdditions...), nil
}

func getDevWorkspaceConfigmaps(namespace string, client runtimeClient.Client) (*v1alpha1.PodAdditions, []corev1.EnvFromSource, error) {
	configmaps := &corev1.ConfigMapList{}
	if err := client.List(context.TODO(), configmaps, k8sclient.InNamespace(namespace), k8sclient.MatchingLabels{
		constants.DevWorkspaceMountLabel: "true",
	}); err != nil {
		return nil, nil, err
	}
	podAdditions := &v1alpha1.PodAdditions{}
	var additionalEnvVars []corev1.EnvFromSource
	for _, configmap := range configmaps.Items {
		mountAs := configmap.Annotations[constants.DevWorkspaceMountAsAnnotation]
		if mountAs == "env" {
			additionalEnvVars = append(additionalEnvVars, getAutoMountConfigMapEnvFromSource(configmap.Name))
		} else {
			mountPath := configmap.Annotations[constants.DevWorkspaceMountPathAnnotation]
			if mountPath == "" {
				mountPath = path.Join("/etc/config/", configmap.Name)
			}
			podAdditions.Volumes = append(podAdditions.Volumes, getAutoMountVolumeWithConfigMap(configmap.Name))
			podAdditions.VolumeMounts = append(podAdditions.VolumeMounts, getAutoMountConfigMapVolumeMount(mountPath, configmap.Name))
		}
	}
	return podAdditions, additionalEnvVars, nil
}

func getDevWorkspaceSecrets(namespace string, client runtimeClient.Client) (*v1alpha1.PodAdditions, []corev1.EnvFromSource, error) {
	secrets := &corev1.SecretList{}
	if err := client.List(context.TODO(), secrets, k8sclient.InNamespace(namespace), k8sclient.MatchingLabels{
		constants.DevWorkspaceMountLabel: "true",
	}); err != nil {
		return nil, nil, err
	}
	podAdditions := &v1alpha1.PodAdditions{}
	var additionalEnvVars []corev1.EnvFromSource
	for _, secret := range secrets.Items {
		mountAs := secret.Annotations[constants.DevWorkspaceMountAsAnnotation]
		if mountAs == "env" {
			additionalEnvVars = append(additionalEnvVars, getAutoMountSecretEnvFromSource(secret.Name))
		} else {
			mountPath := secret.Annotations[constants.DevWorkspaceMountPathAnnotation]
			if mountPath == "" {
				mountPath = path.Join("/etc/", "secret/", secret.Name)
			}
			podAdditions.Volumes = append(podAdditions.Volumes, getAutoMountVolumeWithSecret(secret.Name))
			podAdditions.VolumeMounts = append(podAdditions.VolumeMounts, getAutoMountSecretVolumeMount(mountPath, secret.Name))
		}
	}
	return podAdditions, additionalEnvVars, nil
}

func getAutoMountVolumeWithConfigMap(name string) corev1.Volume {
	modeReadOnly := int32(0640)
	workspaceVolumeMount := corev1.Volume{
		Name: common.AutoMountConfigMapVolumeName(name),
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: name,
				},
				DefaultMode: &modeReadOnly,
			},
		},
	}
	return workspaceVolumeMount
}

func getAutoMountVolumeWithSecret(name string) corev1.Volume {
	modeReadOnly := int32(0640)
	workspaceVolumeMount := corev1.Volume{
		Name: common.AutoMountSecretVolumeName(name),
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  name,
				DefaultMode: &modeReadOnly,
			},
		},
	}
	return workspaceVolumeMount
}

func getAutoMountConfigMapVolumeMount(mountPath, name string) corev1.VolumeMount {
	workspaceVolumeMount := corev1.VolumeMount{
		Name:      common.AutoMountConfigMapVolumeName(name),
		ReadOnly:  true,
		MountPath: mountPath,
	}
	return workspaceVolumeMount
}

func getAutoMountSecretVolumeMount(mountPath, name string) corev1.VolumeMount {
	workspaceVolumeMount := corev1.VolumeMount{
		Name:      common.AutoMountSecretVolumeName(name),
		ReadOnly:  true,
		MountPath: mountPath,
	}
	return workspaceVolumeMount
}

func getAutoMountConfigMapEnvFromSource(name string) corev1.EnvFromSource {
	return corev1.EnvFromSource{
		ConfigMapRef: &corev1.ConfigMapEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: name,
			},
		},
	}
}

func getAutoMountSecretEnvFromSource(name string) corev1.EnvFromSource {
	return corev1.EnvFromSource{
		SecretRef: &corev1.SecretEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: name,
			},
		},
	}
}

func checkAutoMountVolumesForCollision(base, automount []v1alpha1.PodAdditions) error {
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
				return fmt.Errorf("DevWorkspace volume %s conflicts with automounted volume from %s",
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

// formatVolumeDescription formats a given volume as either "configmap <configmap-name>" or "secret <secret-name>",
// depending on whether the volume refers to a configmap or secret. If the volume is neither a secret nor configmap,
// returns the name of the volume itself.
func formatVolumeDescription(vol corev1.Volume) string {
	if vol.Secret != nil {
		return fmt.Sprintf("secret %s", vol.Secret.SecretName)
	} else if vol.ConfigMap != nil {
		return fmt.Sprintf("configmap %s", vol.ConfigMap.Name)
	}
	return vol.Name
}
