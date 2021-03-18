//
// Copyright (c) 2019-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package asyncstorage

import (
	"fmt"
	"strconv"

	"github.com/devfile/devworkspace-operator/internal/images"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// GetAsyncSidecar gets the definition for the async storage sidecar. Within this sidecar, all provided volumes
// are mounted to `/volume.Name`, and the sshVolume is mounted to /etc/ssh/private as read-only.
//
// Note: in the current implementation, the image used for the async sidecar only syncs from ${CHE_PROJECTS_ROOT}
func GetAsyncSidecar(sshVolumeName string, volumes []corev1.Volume) *corev1.Container {
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      sshVolumeName,
			ReadOnly:  true,
			MountPath: "/etc/ssh/private",
		},
	}
	for _, vol := range volumes {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      vol.Name,
			MountPath: fmt.Sprintf("/%s", vol.Name),
		})
	}

	// Note: currently, the async sidecar image only backs up /projects
	container := &corev1.Container{
		Name:  asyncSidecarContainerName,
		Image: images.GetAsyncStorageSidecarImage(),
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 4445,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env: []corev1.EnvVar{
			{
				Name:  "RSYNC_PORT",
				Value: strconv.Itoa(rsyncPort),
			},
		},
		Resources: corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(asyncSidecarMemoryLimit),
			},
			Requests: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(asyncSidecarMemoryRequest),
			},
		},
		VolumeMounts: volumeMounts,
		Lifecycle: &corev1.Lifecycle{
			PreStop: &corev1.Handler{
				Exec: &corev1.ExecAction{
					Command: []string{"/bin/sh", "-c", "/scripts/backup.sh"},
				},
			},
		},
	}
	return container
}
