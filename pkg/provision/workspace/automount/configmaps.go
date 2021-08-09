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
	"context"
	"path"

	corev1 "k8s.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

func getDevWorkspaceConfigmaps(namespace string, client k8sclient.Client) (*v1alpha1.PodAdditions, []corev1.EnvFromSource, error) {
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

func getAutoMountConfigMapVolumeMount(mountPath, name string) corev1.VolumeMount {
	workspaceVolumeMount := corev1.VolumeMount{
		Name:      common.AutoMountConfigMapVolumeName(name),
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
