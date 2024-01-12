//
// Copyright (c) 2019-2024 Red Hat, Inc.
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
	"path"
	"sort"

	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	corev1 "k8s.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/devfile/devworkspace-operator/pkg/constants"
)

func getDevWorkspaceConfigmaps(namespace string, api sync.ClusterAPI) (*Resources, error) {
	configmaps := &corev1.ConfigMapList{}
	if err := api.Client.List(api.Ctx, configmaps, k8sclient.InNamespace(namespace), k8sclient.MatchingLabels{
		constants.DevWorkspaceMountLabel: "true",
	}); err != nil {
		return nil, err
	}
	var allAutoMountResouces []Resources
	for _, configmap := range configmaps.Items {
		if msg := checkAutomountVolumeForPotentialError(&configmap); msg != "" {
			return nil, &dwerrors.FailError{Message: msg}
		}
		mountAs := configmap.Annotations[constants.DevWorkspaceMountAsAnnotation]
		mountPath := configmap.Annotations[constants.DevWorkspaceMountPathAnnotation]
		if mountPath == "" {
			mountPath = path.Join("/etc/config/", configmap.Name)
		}
		accessMode, err := getAccessModeForAutomount(&configmap)
		if err != nil {
			return nil, &dwerrors.FailError{
				Message: fmt.Sprintf("failed to process configmap %s", configmap.Name),
				Err:     err,
			}
		}

		allAutoMountResouces = append(allAutoMountResouces, getAutomountConfigmap(mountPath, mountAs, accessMode, &configmap))
	}
	automountResources := flattenAutomountResources(allAutoMountResouces)
	return &automountResources, nil
}

// getAutomountConfigmap defines the volumes, volumeMounts, and envFromSource that is required to mount
// a given configmap. Parameter mountAs defines how the secret should be mounted (file, subpath, or as env vars).
// Parameter mountPath is ignored when mounting as environment variables
func getAutomountConfigmap(mountPath, mountAs string, accessMode *int32, configmap *corev1.ConfigMap) Resources {
	// Define volume to be used when mountAs is "file" or "subpath"
	volume := corev1.Volume{
		Name: common.AutoMountConfigMapVolumeName(configmap.Name),
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configmap.Name,
				},
				DefaultMode: accessMode,
			},
		},
	}

	// In order to handle access mode when this configmap is merged into a projected volume, we need to add access mode
	// to each item in the configmap. If this configmap does not get merged into a projected volume, these items should be
	// dropped in the final spec -- see dropItemsFieldFromVolumes()
	if accessMode != defaultAccessMode {
		for key := range configmap.Data {
			volume.ConfigMap.Items = append(volume.ConfigMap.Items, corev1.KeyToPath{
				Key:  key,
				Path: key,
				Mode: accessMode,
			})
		}
		for key := range configmap.BinaryData {
			volume.ConfigMap.Items = append(volume.ConfigMap.Items, corev1.KeyToPath{
				Key:  key,
				Path: key,
				Mode: accessMode,
			})
		}
		// Sort to avoid random map iteration order
		sort.Slice(volume.ConfigMap.Items, func(i, j int) bool {
			return volume.ConfigMap.Items[i].Key < volume.ConfigMap.Items[j].Key
		})
	}

	automount := Resources{}
	switch mountAs {
	case constants.DevWorkspaceMountAsEnv:
		envFromSource := corev1.EnvFromSource{
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configmap.Name,
				},
			},
		}
		automount.EnvFromSource = []corev1.EnvFromSource{envFromSource}
	case constants.DevWorkspaceMountAsSubpath:
		var volumeMounts []corev1.VolumeMount
		volumeName := common.AutoMountConfigMapVolumeName(configmap.Name)
		for secretKey := range configmap.Data {
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      volumeName,
				ReadOnly:  true,
				MountPath: path.Join(mountPath, secretKey),
				SubPath:   secretKey,
			})
		}
		for secretKey := range configmap.BinaryData {
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      volumeName,
				ReadOnly:  true,
				MountPath: path.Join(mountPath, secretKey),
				SubPath:   secretKey,
			})
		}
		automount.Volumes = []corev1.Volume{volume}
		automount.VolumeMounts = volumeMounts
	case "", constants.DevWorkspaceMountAsFile:
		volumeMount := corev1.VolumeMount{
			Name:      common.AutoMountConfigMapVolumeName(configmap.Name),
			ReadOnly:  true,
			MountPath: mountPath,
		}
		automount.Volumes = []corev1.Volume{volume}
		automount.VolumeMounts = []corev1.VolumeMount{volumeMount}
	}

	return automount
}
