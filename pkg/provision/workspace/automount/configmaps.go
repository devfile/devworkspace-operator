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
	"path"

	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	corev1 "k8s.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

func getDevWorkspaceConfigmaps(namespace string, api sync.ClusterAPI) (*v1alpha1.PodAdditions, []corev1.EnvFromSource, error) {
	configmaps := &corev1.ConfigMapList{}
	if err := api.Client.List(api.Ctx, configmaps, k8sclient.InNamespace(namespace), k8sclient.MatchingLabels{
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
			podAdditions.Volumes = append(podAdditions.Volumes, GetAutoMountVolumeWithConfigMap(configmap.Name))
			podAdditions.VolumeMounts = append(podAdditions.VolumeMounts, GetAutoMountConfigMapVolumeMount(mountPath, configmap.Name))
		}
	}
	return podAdditions, additionalEnvVars, nil
}

func GetAutoMountVolumeWithConfigMap(name string) corev1.Volume {
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

func GetAutoMountConfigMapVolumeMount(mountPath, name string) corev1.VolumeMount {
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
