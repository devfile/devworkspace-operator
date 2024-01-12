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

	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestCreatesRoleIfNotExists(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	testdw := getTestDevWorkspace("test-devworkspace")
	api := getTestClusterAPI(t, testdw.DevWorkspace)
	err := syncRoles(testdw, api)
	retryErr := &dwerrors.RetryError{}
	if assert.Error(t, err, "Should return RetryError to indicate that role was created") {
		assert.ErrorAs(t, err, &retryErr, "Error should have RetryError type")
	}
	err = syncRoles(testdw, api)
	assert.NoError(t, err, "Should not return error if role is in sync")
	actualRole := &rbacv1.Role{}
	err = api.Client.Get(api.Ctx, types.NamespacedName{
		Name:      common.WorkspaceRoleName(),
		Namespace: testNamespace,
	}, actualRole)
	assert.NoError(t, err, "Role should be created")
}

func TestDoesNothingIfRoleAlreadyInSync(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	testdw := getTestDevWorkspace("test-devworkspace")
	api := getTestClusterAPI(t, testdw.DevWorkspace)
	err := syncRoles(testdw, api)
	retryErr := &dwerrors.RetryError{}
	if assert.Error(t, err, "Should return RetryError to indicate that role was created") {
		assert.ErrorAs(t, err, &retryErr, "Error should have RetryError type")
	}
	err = syncRoles(testdw, api)
	assert.NoError(t, err, "Should not return error if role is in sync")
	actualRole := &rbacv1.Role{}
	err = api.Client.Get(api.Ctx, types.NamespacedName{
		Name:      common.WorkspaceRoleName(),
		Namespace: testNamespace,
	}, actualRole)
	assert.NoError(t, err, "Role should be created")
	err = syncRoles(testdw, api)
	assert.NoError(t, err, "Should not return error if role is in sync")
}

func TestCreatesSCCRoleIfNotExists(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	testdw := getTestDevWorkspaceWithAttributes(t, "test-devworkspace", constants.WorkspaceSCCAttribute, testSCCName)
	api := getTestClusterAPI(t, testdw.DevWorkspace)
	retryErr := &dwerrors.RetryError{}
	err := syncRoles(testdw, api)
	if assert.Error(t, err, "Should return RetryError to indicate that role was created") {
		assert.ErrorAs(t, err, &retryErr, "Error should have RetryError type")
	}
	err = syncRoles(testdw, api)
	if assert.Error(t, err, "Should return RetryError to indicate that SCC role was created") {
		assert.ErrorAs(t, err, &retryErr, "Error should have RetryError type")
	}
	err = syncRoles(testdw, api)
	assert.NoError(t, err, "Should not return error if roles are in sync")
	actualRole := &rbacv1.Role{}
	err = api.Client.Get(api.Ctx, types.NamespacedName{
		Name:      common.WorkspaceSCCRoleName(testSCCName),
		Namespace: testNamespace,
	}, actualRole)
	assert.NoError(t, err, "Role should be created")
}

func TestDoesNothingIfSCCRoleAlreadyInSync(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	testdw := getTestDevWorkspaceWithAttributes(t, "test-devworkspace", constants.WorkspaceSCCAttribute, testSCCName)
	api := getTestClusterAPI(t, testdw.DevWorkspace)
	retryErr := &dwerrors.RetryError{}
	err := syncRoles(testdw, api)
	if assert.Error(t, err, "Should return RetryError to indicate that role was created") {
		assert.ErrorAs(t, err, &retryErr, "Error should have RetryError type")
	}
	err = syncRoles(testdw, api)
	if assert.Error(t, err, "Should return RetryError to indicate that SCC role was created") {
		assert.ErrorAs(t, err, &retryErr, "Error should have RetryError type")
	}
	err = syncRoles(testdw, api)
	assert.NoError(t, err, "Should not return error if roles are in sync")
	actualRole := &rbacv1.Role{}
	err = api.Client.Get(api.Ctx, types.NamespacedName{
		Name:      common.WorkspaceSCCRoleName(testSCCName),
		Namespace: testNamespace,
	}, actualRole)
	assert.NoError(t, err, "Role should be created")
	err = syncRoles(testdw, api)
	assert.NoError(t, err, "Should not return error if role is in sync")
}
