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

package workspace

import (
	workspaceApi "github.com/che-incubator/che-workspace-crd-operator/pkg/apis/workspace/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/config"
	. "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/model"
)

func generatePrerequisites(workspace *workspaceApi.Workspace) ([]runtime.Object, error) {
	pvcStorageQuantity, err := resource.ParseQuantity(PVCStorageSize)
	if err != nil {
		return nil, err
	}

	autoMountServiceAccount := true

	k8sObjects := []runtime.Object{
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ControllerCfg.GetWorkspacePVCName(),
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
				StorageClassName: ControllerCfg.GetPVCStorageClassName(),
			},
		},
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ServiceAccount,
				Namespace: workspace.Namespace,
			},
			AutomountServiceAccountToken: &autoMountServiceAccount,
		},
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "exec",
				Namespace: workspace.Namespace,
			},
			Rules: []rbacv1.PolicyRule{
				{
					Resources: []string{"pods/exec"},
					APIGroups: []string{""},
					Verbs:     []string{"create"},
				},
			},
		},
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "view-workspaces",
				Namespace: workspace.Namespace,
			},
			Rules: []rbacv1.PolicyRule{
				{
					Resources: []string{"workspaces"},
					APIGroups: []string{"workspace.che.eclipse.org"},
					Verbs:     []string{"get", "list"},
				},
			},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ServiceAccount + "-view",
				Namespace: workspace.Namespace,
			},
			RoleRef: rbacv1.RoleRef{
				Kind: "ClusterRole",
				Name: "view",
			},
			Subjects: []rbacv1.Subject{
				rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      ServiceAccount,
					Namespace: workspace.Namespace,
				},
			},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ServiceAccount + "-exec",
				Namespace: workspace.Namespace,
			},
			RoleRef: rbacv1.RoleRef{
				Kind: "Role",
				Name: "exec",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      ServiceAccount,
					Namespace: workspace.Namespace,
				},
			},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ServiceAccount + "-view-workspaces",
				Namespace: workspace.Namespace,
			},
			RoleRef: rbacv1.RoleRef{
				Kind: "Role",
				Name: "view-workspaces",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      ServiceAccount,
					Namespace: workspace.Namespace,
				},
			},
		},
	}
	return k8sObjects, nil
}
