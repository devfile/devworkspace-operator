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
	"path"

	corev1 "k8s.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

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
			additionalEnvVars = append(additionalEnvVars, getConfigMapEnvFromSource(configmap.Name))
		} else {
			mountPath := configmap.Annotations[constants.DevWorkspaceMountPathAnnotation]
			if mountPath == "" {
				mountPath = path.Join("/etc/config/", configmap.Name)
			}
			podAdditions.Volumes = append(podAdditions.Volumes, getVolumeWithConfigMap(configmap.Name))
			podAdditions.VolumeMounts = append(podAdditions.VolumeMounts, getVolumeMounts(mountPath, configmap.Name))
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
			additionalEnvVars = append(additionalEnvVars, getSecretEnvFromSource(secret.Name))
		} else {
			mountPath := secret.Annotations[constants.DevWorkspaceMountPathAnnotation]
			if mountPath == "" {
				mountPath = path.Join("/etc/", "secret/", secret.Name)
			}
			podAdditions.Volumes = append(podAdditions.Volumes, getVolumeWithSecret(secret.Name))
			podAdditions.VolumeMounts = append(podAdditions.VolumeMounts, getVolumeMounts(mountPath, secret.Name))
		}
	}
	return podAdditions, additionalEnvVars, nil
}

func getVolumeWithConfigMap(name string) corev1.Volume {
	modeReadOnly := int32(0640)
	workspaceVolumeMount := corev1.Volume{
		Name: name,
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

func getVolumeWithSecret(name string) corev1.Volume {
	modeReadOnly := int32(0640)
	workspaceVolumeMount := corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  name,
				DefaultMode: &modeReadOnly,
			},
		},
	}
	return workspaceVolumeMount
}

func getVolumeMounts(mountPath, name string) corev1.VolumeMount {
	workspaceVolumeMount := corev1.VolumeMount{
		Name:      name,
		ReadOnly:  true,
		MountPath: mountPath,
	}
	return workspaceVolumeMount
}

func getConfigMapEnvFromSource(name string) corev1.EnvFromSource {
	return corev1.EnvFromSource{
		ConfigMapRef: &corev1.ConfigMapEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: name,
			},
		},
	}
}

func getSecretEnvFromSource(name string) corev1.EnvFromSource {
	return corev1.EnvFromSource{
		SecretRef: &corev1.SecretEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: name,
			},
		},
	}
}
