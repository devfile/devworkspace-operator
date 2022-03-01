//
// Copyright (c) 2019-2022 Red Hat, Inc.
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
	"errors"
	"path"

	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	corev1 "k8s.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/devfile/devworkspace-operator/pkg/constants"
)

func getDevWorkspaceSecrets(namespace string, api sync.ClusterAPI) (*Resources, error) {
	secrets := &corev1.SecretList{}
	if err := api.Client.List(api.Ctx, secrets, k8sclient.InNamespace(namespace), k8sclient.MatchingLabels{
		constants.DevWorkspaceMountLabel: "true",
	}); err != nil {
		return nil, err
	}
	var allAutoMountResouces []Resources
	for _, secret := range secrets.Items {
		if msg := checkAutomountVolumeForPotentialError(&secret); msg != "" {
			return nil, &AutoMountError{Err: errors.New(msg), IsFatal: true}
		}
		mountAs := secret.Annotations[constants.DevWorkspaceMountAsAnnotation]
		mountPath := secret.Annotations[constants.DevWorkspaceMountPathAnnotation]
		if mountPath == "" {
			mountPath = path.Join("/etc/", "secret/", secret.Name)
		}
		allAutoMountResouces = append(allAutoMountResouces, getAutomountSecret(mountPath, mountAs, &secret))
	}
	automountResources := flattenAutomountResources(allAutoMountResouces)
	return &automountResources, nil
}

// getAutomountSecret defines the volumes, volumeMounts, and envFromSource that is required to mount
// a given secret. Parameter mountAs defines how the secret should be mounted (file, subpath, or as env vars).
// Parameter mountPath is ignored when mounting as environment variables
func getAutomountSecret(mountPath, mountAs string, secret *corev1.Secret) Resources {
	// Define volume to be used when mountAs is "file" or "subpath"
	volume := corev1.Volume{
		Name: common.AutoMountSecretVolumeName(secret.Name),
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  secret.Name,
				DefaultMode: &modeReadOnly,
			},
		},
	}

	automount := Resources{}
	switch mountAs {
	case constants.DevWorkspaceMountAsEnv:
		envFromSource := corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: secret.Name,
				},
			},
		}
		automount.EnvFromSource = []corev1.EnvFromSource{envFromSource}
	case constants.DevWorkspaceMountAsSubpath:
		var volumeMounts []corev1.VolumeMount
		for secretKey := range secret.Data {
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      common.AutoMountSecretVolumeName(secret.Name),
				ReadOnly:  true,
				MountPath: path.Join(mountPath, secretKey),
				SubPath:   secretKey,
			})
		}
		automount.Volumes = []corev1.Volume{volume}
		automount.VolumeMounts = volumeMounts
	case "", constants.DevWorkspaceMountAsFile:
		volumeMount := corev1.VolumeMount{
			Name:      common.AutoMountSecretVolumeName(secret.Name),
			ReadOnly:  true,
			MountPath: mountPath,
		}
		automount.Volumes = []corev1.Volume{volume}
		automount.VolumeMounts = []corev1.VolumeMount{volumeMount}
	}

	return automount
}
