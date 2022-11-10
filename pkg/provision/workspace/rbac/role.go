// Copyright (c) 2019-2022 Red Hat, Inc.
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

package rbac

import (
	"fmt"

	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"

	rbacv1 "k8s.io/api/rbac/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func syncRoles(workspace *common.DevWorkspaceWithConfig, api sync.ClusterAPI) error {
	defaultRole := generateDefaultRole(workspace.Namespace)
	if _, err := sync.SyncObjectWithCluster(defaultRole, api); err != nil {
		return wrapSyncError(err)
	}
	if !workspace.Spec.Template.Attributes.Exists(constants.WorkspaceSCCAttribute) {
		return nil
	}
	sccName := workspace.Spec.Template.Attributes.GetString(constants.WorkspaceSCCAttribute, nil)
	sccRole := generateUseRoleForSCC(workspace.Namespace, sccName)
	if _, err := sync.SyncObjectWithCluster(sccRole, api); err != nil {
		return wrapSyncError(err)
	}
	return nil
}

func deleteRole(name, namespace string, api sync.ClusterAPI) error {
	role := &rbacv1.Role{}
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	err := api.Client.Get(api.Ctx, namespacedName, role)
	switch {
	case err == nil:
		if err := api.Client.Delete(api.Ctx, role); err != nil {
			return &RetryError{fmt.Errorf("failed to delete role %s in namespace %s: %w", name, namespace, err)}
		}
		return &RetryError{fmt.Errorf("deleted role %s in namespace %s", name, namespace)}
	case k8sErrors.IsNotFound(err):
		// Already deleted
		return nil
	default:
		return err
	}
}

func generateDefaultRole(namespace string) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.WorkspaceRoleName(),
			Namespace: namespace,
			Labels:    rbacLabels,
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
				Resources: []string{"pods"},
				APIGroups: []string{"metrics.k8s.io"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				Resources: []string{"deployments", "replicasets"},
				APIGroups: []string{"apps", "extensions"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				Resources:     []string{"secrets"},
				APIGroups:     []string{""},
				Verbs:         []string{"get", "create", "patch", "delete"},
				ResourceNames: []string{"workspace-credentials-secret"},
			},
			{
				Resources:     []string{"configmaps"},
				APIGroups:     []string{""},
				Verbs:         []string{"get", "create", "patch", "delete"},
				ResourceNames: []string{"workspace-preferences-configmap"},
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
	}
}

func generateUseRoleForSCC(namespace, sccName string) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.WorkspaceSCCRoleName(sccName),
			Namespace: namespace,
			Labels:    rbacLabels,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Resources:     []string{"securitycontextconstraints"},
				APIGroups:     []string{"security.openshift.io"},
				Verbs:         []string{"use"},
				ResourceNames: []string{sccName},
			},
		},
	}
}
