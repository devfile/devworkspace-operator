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
	"testing"

	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

func TestRemovesOldRBACWhenNewRBACNotPresent(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	api := getTestClusterAPI(t, oldRole, oldRolebinding)
	// Expect three calls to be required: 1. delete role, 2. delete rolebinding, 3. return nil
	err := cleanupDeprecatedRBAC(testNamespace, api)
	retryErr := &dwerrors.RetryError{}
	if assert.ErrorAs(t, err, &retryErr, "Error should be of type RetryErr") {
		assert.Contains(t, err.Error(), "deleted deprecated DevWorkspace Role")
	}
	err = cleanupDeprecatedRBAC(testNamespace, api)
	if assert.ErrorAs(t, err, &retryErr, "Error should be of type RetryErr") {
		assert.Contains(t, err.Error(), "deleted deprecated DevWorkspace RoleBinding")
	}
	err = cleanupDeprecatedRBAC(testNamespace, api)
	assert.NoError(t, err, "Should not return error if old rbac does not exist")

	err = api.Client.Get(api.Ctx, types.NamespacedName{
		Name:      oldRole.Name,
		Namespace: testNamespace,
	}, &rbacv1.Role{})
	if assert.Error(t, err, "Expect get old role to return IsNotFound error") {
		assert.True(t, k8sErrors.IsNotFound(err), "Expect error to have IsNotFound type")
	}

	err = api.Client.Get(api.Ctx, types.NamespacedName{
		Name:      oldRolebinding.Name,
		Namespace: testNamespace,
	}, &rbacv1.RoleBinding{})
	if assert.Error(t, err, "Expect get old role to return IsNotFound error") {
		assert.True(t, k8sErrors.IsNotFound(err), "Expect error to have IsNotFound type")
	}
}

func TestDoesNotRemoveOldRBACWhenNewRBACPresent(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	api := getTestClusterAPI(t, oldRole, oldRolebinding, newRole, newRolebinding)
	err := cleanupDeprecatedRBAC(testNamespace, api)
	assert.NoError(t, err, "Should do nothing if new RBAC role/rolebinding are present")

	err = api.Client.Get(api.Ctx, types.NamespacedName{
		Name:      oldRole.Name,
		Namespace: testNamespace,
	}, &rbacv1.Role{})
	assert.NoError(t, err, "Old role should still exist if present initially")

	err = api.Client.Get(api.Ctx, types.NamespacedName{
		Name:      oldRolebinding.Name,
		Namespace: testNamespace,
	}, &rbacv1.RoleBinding{})
	assert.NoError(t, err, "Old rolebinding should still exist if present initially")
}

func TestRemovesOldRolebindingWhenNewRolebindingNotPresent(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	api := getTestClusterAPI(t, oldRole, oldRolebinding, newRole)
	// Expect two calls to be required: 1. delete rolebinding, 2. return nil
	err := cleanupDeprecatedRBAC(testNamespace, api)
	retryErr := &dwerrors.RetryError{}
	if assert.ErrorAs(t, err, &retryErr, "Error should be of type RetryErr") {
		assert.Contains(t, err.Error(), "deleted deprecated DevWorkspace RoleBinding")
	}
	err = cleanupDeprecatedRBAC(testNamespace, api)
	assert.NoError(t, err, "Should not return error if old rbac does not exist")

	err = api.Client.Get(api.Ctx, types.NamespacedName{
		Name:      oldRole.Name,
		Namespace: testNamespace,
	}, &rbacv1.Role{})
	assert.NoError(t, err, "Old role should still exist if present initially")

	err = api.Client.Get(api.Ctx, types.NamespacedName{
		Name:      oldRolebinding.Name,
		Namespace: testNamespace,
	}, &rbacv1.RoleBinding{})
	if assert.Error(t, err, "Expect get old role to return IsNotFound error") {
		assert.True(t, k8sErrors.IsNotFound(err), "Expect error to have IsNotFound type")
	}
}

func TestRemovesOldRoleWhenNewRoleNotPresent(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	api := getTestClusterAPI(t, oldRole, oldRolebinding, newRolebinding)
	// Expect two calls to be required: 1. delete role, 2. return nil
	err := cleanupDeprecatedRBAC(testNamespace, api)
	retryErr := &dwerrors.RetryError{}
	if assert.ErrorAs(t, err, &retryErr, "Error should be of type RetryErr") {
		assert.Contains(t, err.Error(), "deleted deprecated DevWorkspace Role")
	}
	err = cleanupDeprecatedRBAC(testNamespace, api)
	assert.NoError(t, err, "Should not return error if old rbac does not exist")

	err = api.Client.Get(api.Ctx, types.NamespacedName{
		Name:      oldRole.Name,
		Namespace: testNamespace,
	}, &rbacv1.Role{})
	if assert.Error(t, err, "Expect get old role to return IsNotFound error") {
		assert.True(t, k8sErrors.IsNotFound(err), "Expect error to have IsNotFound type")
	}

	err = api.Client.Get(api.Ctx, types.NamespacedName{
		Name:      oldRolebinding.Name,
		Namespace: testNamespace,
	}, &rbacv1.RoleBinding{})
	assert.NoError(t, err, "Old rolebinding should still exist if present initially")
}
