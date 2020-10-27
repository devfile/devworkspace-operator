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

package provision

import (
	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha1"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SyncPVC(workspace *devworkspace.DevWorkspace, components []v1alpha1.ComponentDescription, client client.Client, reqLogger logr.Logger) ProvisioningStatus {
	if !IsPVCRequired(components) {
		return ProvisioningStatus{Continue: true}
	}

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

// IsPVCRequired checks to see if we need a PVC for the given devfile.
// If there is any Containers with VolumeMounts that have the same name as the workspace PVC name then we need a PVC
func IsPVCRequired(components []v1alpha1.ComponentDescription) bool {
	volumeName := config.ControllerCfg.GetWorkspacePVCName()
	for _, comp := range components {
		for _, cont := range comp.PodAdditions.Containers {
			for _, vm := range cont.VolumeMounts {
				if vm.Name == volumeName {
					return true
				}
			}
		}
	}
	return false
}
