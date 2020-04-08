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
	"strings"

	"github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/prerequisites"

	"github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	"github.com/che-incubator/che-workspace-operator/pkg/config"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IsPVCRequired checks to see if we need a PVC for the given devfile. If cloud-shell is present in the devfile,
// we do not need PVC else we do
func IsPVCRequired(components []v1alpha1.ComponentSpec) bool {
	for _, comp := range components {
		if strings.Contains(comp.Id, config.CloudShellID) && comp.Type == v1alpha1.CheEditor {
			return false
		}
	}
	return true
}

// GeneratePVC creates a new PVC that will be mounted into the workspace
func GeneratePVC(workspace *v1alpha1.Workspace, client client.Client, reqLogger logr.Logger) error {

	pvcStorageQuantity, err := resource.ParseQuantity(config.PVCStorageSize)
	if err != nil {
		return err
	}

	pvc := []runtime.Object{&corev1.PersistentVolumeClaim{
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
	}}

	prereq := pvc[0]
	prereqAsMetaObject, _ := prereq.(metav1.Object)

	err = prerequisites.ProvisionPrereqs(prereq, prereqAsMetaObject, client, reqLogger)
	if err != nil {
		return err
	}

	return nil
}
