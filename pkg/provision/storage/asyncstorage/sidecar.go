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
func GetAsyncSidecar(devworkspaceID, sshVolumeName string, volumes []corev1.Volume) *corev1.Container {
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
			{
				Name:  "CHE_PROJECTS_ROOT",
				Value: "/projects",
			},
			{
				Name:  "CHE_WORKSPACE_ID",
				Value: devworkspaceID,
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
			PreStop: &corev1.LifecycleHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"/bin/sh", "-c", "/scripts/backup.sh"},
				},
			},
		},
	}
	return container
}
