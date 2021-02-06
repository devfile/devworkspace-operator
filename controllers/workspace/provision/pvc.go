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

package provision

import (
	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SyncPVC(workspace *devworkspace.DevWorkspace, client client.Client, reqLogger logr.Logger) ProvisioningStatus {
	pvc, err := generatePVC(workspace)
	if err != nil {
		return ProvisioningStatus{Err: err}
	}

	didChange, err := SyncObject(pvc, client, reqLogger, false)
	return ProvisioningStatus{Continue: !didChange, Err: err}
}

func generatePVC(workspace *devworkspace.DevWorkspace) (*corev1.PersistentVolumeClaim, error) {
	pvcStorageQuantity, err := resource.ParseQuantity(config.PVCStorageSize)
	if err != nil {
		return nil, err
	}

	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ControllerCfg.GetWorkspacePVCName(),
			Namespace: workspace.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": pvcStorageQuantity,
				},
			},
			StorageClassName: config.ControllerCfg.GetPVCStorageClassName(),
		},
	}, nil
}
