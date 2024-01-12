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
	"path"

	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	corev1 "k8s.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

func getAutoMountPVCs(namespace string, api sync.ClusterAPI) (*Resources, error) {
	pvcs := &corev1.PersistentVolumeClaimList{}
	if err := api.Client.List(api.Ctx, pvcs, k8sclient.InNamespace(namespace), k8sclient.MatchingLabels{
		constants.DevWorkspaceMountLabel: "true",
	}); err != nil {
		return nil, err
	}
	if len(pvcs.Items) == 0 {
		return nil, nil
	}

	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount
	for _, pvc := range pvcs.Items {
		mountPath := pvc.Annotations[constants.DevWorkspaceMountPathAnnotation]
		if mountPath == "" {
			mountPath = path.Join("/tmp/", pvc.Name)
		}

		mountReadOnly := false
		if pvc.Annotations[constants.DevWorkspaceMountReadyOnlyAnnotation] == "true" {
			mountReadOnly = true
		}

		volumes = append(volumes, corev1.Volume{
			Name: common.AutoMountPVCVolumeName(pvc.Name),
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvc.Name,
					ReadOnly:  mountReadOnly,
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      common.AutoMountPVCVolumeName(pvc.Name),
			MountPath: mountPath,
		})
	}
	return &Resources{
		Volumes:      volumes,
		VolumeMounts: volumeMounts,
	}, nil
}
