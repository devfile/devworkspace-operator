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
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	rbacv1 "k8s.io/api/rbac/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// cleanupDeprecatedRBAC removes old Roles and RoleBindings created by an earlier version
// of the DevWorkspace Operator. These earlier roles and rolebindings are no longer used
// and need to be removed directly as there is no usual mechanism for their removal.
// TODO: Remove this functionality for DevWorkspace Operator v0.19
func cleanupDeprecatedRBAC(namespace string, api sync.ClusterAPI) error {
	role := &rbacv1.Role{}
	roleNN := types.NamespacedName{
		Name:      common.OldWorkspaceRoleName(),
		Namespace: namespace,
	}
	err := api.Client.Get(api.Ctx, roleNN, role)
	switch {
	case err == nil:
		if err := api.Client.Delete(api.Ctx, role); err != nil {
			return err
		}
		return &RetryError{fmt.Errorf("deleted deprecated DevWorkspace Role")}
	case k8sErrors.IsNotFound(err):
		break
	default:
		return err
	}
	rolebinding := &rbacv1.RoleBinding{}
	rolebindingNN := types.NamespacedName{
		Name:      common.OldWorkspaceRolebindingName(),
		Namespace: namespace,
	}
	err = api.Client.Get(api.Ctx, rolebindingNN, rolebinding)
	switch {
	case err == nil:
		if err := api.Client.Delete(api.Ctx, rolebinding); err != nil {
			return err
		}
		return &RetryError{fmt.Errorf("deleted deprecated DevWorkspace RoleBinding")}
	case k8sErrors.IsNotFound(err):
		break
	default:
		return err
	}
	return nil
}
