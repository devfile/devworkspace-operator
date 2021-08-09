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

func getAutoMountPVCs(namespace string, client k8sclient.Client) (*v1alpha1.PodAdditions, error) {
	pvcs := &corev1.PersistentVolumeClaimList{}
	if err := client.List(context.TODO(), pvcs, k8sclient.InNamespace(namespace), k8sclient.MatchingLabels{
		constants.DevWorkspaceMountLabel: "true",
	}); err != nil {
		return nil, err
	}
	if len(pvcs.Items) == 0 {
		return nil, nil
	}

	podAdditions := &v1alpha1.PodAdditions{}
	for _, pvc := range pvcs.Items {
		mountPath := pvc.Annotations[constants.DevWorkspaceMountPathAnnotation]
		if mountPath == "" {
			mountPath = path.Join("/tmp/", pvc.Name)
		}

		mountReadOnly := false
		if pvc.Annotations[constants.DevWorkspaceMountReadyOnlyAnnotation] == "true" {
			mountReadOnly = true
		}

		podAdditions.Volumes = append(podAdditions.Volumes, corev1.Volume{
			Name: common.AutoMountPVCVolumeName(pvc.Name),
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvc.Name,
					ReadOnly:  mountReadOnly,
				},
			},
		})
		podAdditions.VolumeMounts = append(podAdditions.VolumeMounts, corev1.VolumeMount{
			Name:      common.AutoMountPVCVolumeName(pvc.Name),
			MountPath: mountPath,
		})
	}
	return podAdditions, nil
}
