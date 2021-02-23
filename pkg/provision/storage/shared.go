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

package storage

import (
	"errors"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"

	"github.com/devfile/devworkspace-operator/controllers/workspace/provision"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	devfileConstants "github.com/devfile/devworkspace-operator/pkg/library/constants"
	containerlib "github.com/devfile/devworkspace-operator/pkg/library/container"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getCommonPVCSpec(namespace string) (*corev1.PersistentVolumeClaim, error) {
	pvcStorageQuantity, err := resource.ParseQuantity(constants.PVCStorageSize)
	if err != nil {
		return nil, err
	}

	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ControllerCfg.GetWorkspacePVCName(),
			Namespace: namespace,
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

// needsStorage returns true if storage will need to be provisioned for the current workspace. Note that ephemeral volumes
// do not need to provision storage
func needsStorage(workspace *dw.DevWorkspaceTemplateSpec) bool {
	projectsVolumeIsEphemeral := false
	for _, component := range workspace.Components {
		if component.Volume != nil {
			// If any non-ephemeral volumes are defined, we need to mount storage
			if !component.Volume.Ephemeral {
				return true
			}
			if component.Name == devfileConstants.ProjectsVolumeName {
				projectsVolumeIsEphemeral = component.Volume.Ephemeral
			}
		}
	}
	if projectsVolumeIsEphemeral {
		// No non-ephemeral volumes, and projects volume mount is ephemeral, so all volumes are ephemeral
		return false
	}
	// Implicit projects volume is non-ephemeral, so any container that mounts sources requires storage
	return containerlib.AnyMountSources(workspace.Components)
}

func syncCommonPVC(namespace string, clusterAPI provision.ClusterAPI) (*corev1.PersistentVolumeClaim, error) {
	pvc, err := getCommonPVCSpec(namespace)
	if err != nil {
		return nil, err
	}
	currObject, didChange, err := provision.SyncObject(pvc, clusterAPI.Client, clusterAPI.Logger, false)
	if err != nil {
		return nil, err
	}
	if didChange {
		return nil, &NotReadyError{
			Message: "Updated common PVC on cluster",
		}
	}
	currPVC, ok := currObject.(*corev1.PersistentVolumeClaim)
	if !ok {
		return nil, errors.New("tried to sync PVC to cluster but did not get a PVC back")
	}
	if currPVC.Status.Phase != corev1.ClaimBound {
		return nil, &NotReadyError{
			Message: "Common PVC is not bound to a volume",
		}
	}
	return currPVC, nil
}
