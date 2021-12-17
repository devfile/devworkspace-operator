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

	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

func getDevWorkspaceConfigmaps(namespace string, api sync.ClusterAPI) (*automountResources, error) {
	configmaps := &corev1.ConfigMapList{}
	if err := api.Client.List(api.Ctx, configmaps, k8sclient.InNamespace(namespace), k8sclient.MatchingLabels{
		constants.DevWorkspaceMountLabel: "true",
	}); err != nil {
		return nil, err
	}
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount
	var additionalEnvVars []corev1.EnvFromSource
	for _, configmap := range configmaps.Items {
		mountAs := configmap.Annotations[constants.DevWorkspaceMountAsAnnotation]
		if mountAs == "env" {
			additionalEnvVars = append(additionalEnvVars, getAutoMountConfigMapEnvFromSource(configmap.Name))
			continue
		}
		mountPath := configmap.Annotations[constants.DevWorkspaceMountPathAnnotation]
		if mountPath == "" {
			mountPath = path.Join("/etc/config/", configmap.Name)
		}
		if mountAs == "subpath" {
			volumes = append(volumes, getAutoMountVolumeWithConfigMap(configmap.Name))
			volumeMounts = append(volumeMounts, getAutoMountConfigMapSubpathVolumeMounts(mountPath, configmap)...)
		} else {
			// mountAs == "file", "", or anything else (default). Don't treat invalid values as errors to avoid
			// failing all workspace starts in this namespace
			volumes = append(volumes, getAutoMountVolumeWithConfigMap(configmap.Name))
			volumeMounts = append(volumeMounts, getAutoMountConfigMapVolumeMount(mountPath, configmap.Name))
		}
	}
	return &automountResources{
		Volumes:       volumes,
		VolumeMounts:  volumeMounts,
		EnvFromSource: additionalEnvVars,
	}, nil
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

func getAutoMountConfigMapSubpathVolumeMounts(mountPath string, cm corev1.ConfigMap) []corev1.VolumeMount {
	var workspaceVolumeMounts []corev1.VolumeMount
	for configmapKey := range cm.Data {
		workspaceVolumeMounts = append(workspaceVolumeMounts, corev1.VolumeMount{
			Name:      common.AutoMountConfigMapVolumeName(cm.Name),
			ReadOnly:  true,
			MountPath: path.Join(mountPath, configmapKey),
			SubPath:   configmapKey,
		})
	}
	return workspaceVolumeMounts
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
