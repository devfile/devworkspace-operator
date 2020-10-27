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
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SyncRBAC generates RBAC and synchronizes the runtime objects
func SyncRBAC(workspace *devworkspace.DevWorkspace, client client.Client, reqLogger logr.Logger) ProvisioningStatus {
	rbac := generateRBAC(workspace.Namespace)

	didChange, err := SyncMutableObjects(rbac, client, reqLogger)
	return ProvisioningStatus{Continue: !didChange, Err: err}
}

func generateRBAC(namespace string) []runtime.Object {
	// TODO: The rolebindings here are created namespace-wide; find a way to limit this, given that each workspace
	return []runtime.Object{
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "workspace",
				Namespace: namespace,
			},
			Rules: []rbacv1.PolicyRule{
				{
					Resources: []string{"pods/exec"},
					APIGroups: []string{""},
					Verbs:     []string{"create"},
				},
				{
					Resources: []string{"pods"},
					APIGroups: []string{""},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					Resources: []string{"deployments", "replicasets"},
					APIGroups: []string{"apps", "extensions"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					Resources: []string{"devworkspaces"},
					APIGroups: []string{"workspace.devfile.io"},
					Verbs:     []string{"patch"},
				},
			},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      config.ServiceAccount + "-workspace",
				Namespace: namespace,
			},
			RoleRef: rbacv1.RoleRef{
				Kind: "Role",
				Name: "workspace",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "Group",
					Name: "system:serviceaccounts:" + namespace,
				},
			},
		},
	}
}
