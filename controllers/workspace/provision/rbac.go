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
	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"

	"github.com/devfile/devworkspace-operator/pkg/constants"

	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SyncRBAC generates RBAC and synchronizes the runtime objects
func SyncRBAC(workspace *dw.DevWorkspace, client client.Client, reqLogger logr.Logger) ProvisioningStatus {
	rbac := generateRBAC(workspace.Namespace)

	requeue, err := SyncMutableObjects(rbac, client, reqLogger)
	return ProvisioningStatus{Continue: !requeue, Err: err}
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
					Resources: []string{"secrets"},
					APIGroups: []string{""},
					Verbs:     []string{"get", "list", "create", "delete"},
				},
				{
					Resources: []string{"devworkspaces"},
					APIGroups: []string{"workspace.devfile.io"},
					Verbs:     []string{"get", "watch", "list", "patch", "update"},
				},
				{
					Resources: []string{"devworkspaceroutings"},
					APIGroups: []string{"controller.devfile.io"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					Resources: []string{"devworkspacetemplates"},
					APIGroups: []string{"workspace.devfile.io"},
					Verbs:     []string{"get", "create", "patch", "update", "delete", "list", "watch"},
				},
			},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.ServiceAccount + "-dw",
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
