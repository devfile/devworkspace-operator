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

package rbac

import (
	"fmt"

	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	rbacv1 "k8s.io/api/rbac/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// cleanupDeprecatedRBAC removes old Roles and RoleBindings created by an earlier version
// of the DevWorkspace Operator. These earlier roles and rolebindings are no longer used
// and need to be removed directly as there is no usual mechanism for their removal.
//
// Since the cache filters used for the operator are label-based and the old roles/bindings
// do not have the appropriate labels, the old role/binding are "invisible" to the controller
// This means we have to delete the object without reading it first. To avoid submitting many
// delete requests to the API, we only do this if the new role/binding are not present.
// TODO: Remove this functionality for DevWorkspace Operator v0.19
func cleanupDeprecatedRBAC(namespace string, api sync.ClusterAPI) error {
	newRole := &rbacv1.Role{}
	newRoleNN := types.NamespacedName{
		Name:      common.WorkspaceRoleName(),
		Namespace: namespace,
	}
	oldRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.OldWorkspaceRoleName(),
			Namespace: namespace,
		},
	}
	err := api.Client.Get(api.Ctx, newRoleNN, newRole)
	switch {
	case err == nil:
		// New role exists, don't try to delete old role
		break
	case k8sErrors.IsNotFound(err):
		// Try to delete old role
		deleteErr := api.Client.Delete(api.Ctx, oldRole)
		switch {
		case deleteErr == nil:
			return &dwerrors.RetryError{Message: fmt.Sprintf("deleted deprecated DevWorkspace Role")}
		case k8sErrors.IsNotFound(err):
			// Already deleted
			break
		default:
			return deleteErr
		}
	default:
		return err
	}

	newRolebinding := &rbacv1.RoleBinding{}
	newRolebindingNN := types.NamespacedName{
		Name:      common.WorkspaceRolebindingName(),
		Namespace: namespace,
	}
	oldRolebinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.OldWorkspaceRolebindingName(),
			Namespace: namespace,
		},
	}
	err = api.Client.Get(api.Ctx, newRolebindingNN, newRolebinding)
	switch {
	case err == nil:
		// New role exists, don't try to delete old role
		break
	case k8sErrors.IsNotFound(err):
		// Try to delete old role
		deleteErr := api.Client.Delete(api.Ctx, oldRolebinding)
		switch {
		case deleteErr == nil:
			return &dwerrors.RetryError{Message: fmt.Sprintf("deleted deprecated DevWorkspace RoleBinding")}
		case k8sErrors.IsNotFound(err):
			// Already deleted
			break
		default:
			return deleteErr
		}
	default:
		return err
	}

	return nil
}
