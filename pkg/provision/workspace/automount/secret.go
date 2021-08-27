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

	v1 "k8s.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

func getDevWorkspaceSecrets(namespace string, client k8sclient.Client) (*v1alpha1.PodAdditions, []v1.EnvFromSource, error) {
	secrets := &v1.SecretList{}
	if err := client.List(context.TODO(), secrets, k8sclient.InNamespace(namespace), k8sclient.MatchingLabels{
		constants.DevWorkspaceMountLabel: "true",
	}); err != nil {
		return nil, nil, err
	}
	podAdditions := &v1alpha1.PodAdditions{}
	var additionalEnvVars []v1.EnvFromSource
	for _, secret := range secrets.Items {
		mountAs := secret.Annotations[constants.DevWorkspaceMountAsAnnotation]
		if mountAs == "env" {
			additionalEnvVars = append(additionalEnvVars, getAutoMountSecretEnvFromSource(secret.Name))
		} else {
			mountPath := secret.Annotations[constants.DevWorkspaceMountPathAnnotation]
			if mountPath == "" {
				mountPath = path.Join("/etc/", "secret/", secret.Name)
			}
			podAdditions.Volumes = append(podAdditions.Volumes, GetAutoMountVolumeWithSecret(secret.Name))
			podAdditions.VolumeMounts = append(podAdditions.VolumeMounts, GetAutoMountSecretVolumeMount(mountPath, secret.Name))
		}
	}
	return podAdditions, additionalEnvVars, nil
}

func GetAutoMountVolumeWithSecret(name string) v1.Volume {
	modeReadOnly := int32(0640)
	workspaceVolumeMount := v1.Volume{
		Name: common.AutoMountSecretVolumeName(name),
		VolumeSource: v1.VolumeSource{
			Secret: &v1.SecretVolumeSource{
				SecretName:  name,
				DefaultMode: &modeReadOnly,
			},
		},
	}
	return workspaceVolumeMount
}

func GetAutoMountSecretVolumeMount(mountPath, name string) v1.VolumeMount {
	workspaceVolumeMount := v1.VolumeMount{
		Name:      common.AutoMountSecretVolumeName(name),
		ReadOnly:  true,
		MountPath: mountPath,
	}
	return workspaceVolumeMount
}

func getAutoMountSecretEnvFromSource(name string) v1.EnvFromSource {
	return v1.EnvFromSource{
		SecretRef: &v1.SecretEnvSource{
			LocalObjectReference: v1.LocalObjectReference{
				Name: name,
			},
		},
	}
}
