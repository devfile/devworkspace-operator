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

func TestCreatesRolebindingIfNotExists(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	testdw := getTestDevWorkspace("test-devworkspace")
	api := getTestClusterAPI(t, testdw.DevWorkspace)
	err := syncRolebindings(testdw, api)
	retryErr := &dwerrors.RetryError{}
	if assert.Error(t, err, "Should return RetryError to indicate that rolebinding was created") {
		assert.ErrorAs(t, err, &retryErr, "Error should have RetryError type")
	}
	err = syncRolebindings(testdw, api)
	assert.NoError(t, err, "Should not return error if rolebinding is in sync")
	actualRB := &rbacv1.RoleBinding{}
	err = api.Client.Get(api.Ctx, types.NamespacedName{
		Name:      common.WorkspaceRolebindingName(),
		Namespace: testNamespace,
	}, actualRB)
	assert.NoError(t, err, "Rolebinding should be created")
	assert.Equal(t, common.WorkspaceRoleName(), actualRB.RoleRef.Name, "Rolebinding shold reference default role")
	expectedSAName := common.ServiceAccountName(testdw)
	assert.True(t, testHasSubject(expectedSAName, testNamespace, actualRB), "Created rolebinding should have workspace SA as subject")
}

func TestAddsMultipleSubjectsToRolebinding(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	testdw := getTestDevWorkspace("test-devworkspace")
	testdw2 := getTestDevWorkspace("test-devworkspace-2")
	api := getTestClusterAPI(t, testdw.DevWorkspace)
	err := syncRolebindings(testdw, api)
	retryErr := &dwerrors.RetryError{}
	if assert.Error(t, err, "Should return RetryError to indicate that rolebinding was created") {
		assert.ErrorAs(t, err, &retryErr, "Error should have RetryError type")
	}
	err = syncRolebindings(testdw, api)
	assert.NoError(t, err, "Should not return error if rolebinding is in sync")
	err = syncRolebindings(testdw2, api)
	if assert.Error(t, err, "Should return RetryError to indicate that rolebinding was updated") {
		assert.ErrorAs(t, err, &retryErr, "Error should have RetryError type")
	}
	err = syncRolebindings(testdw2, api)
	assert.NoError(t, err, "Should not return error if rolebinding is in sync")

	actualRB := &rbacv1.RoleBinding{}
	err = api.Client.Get(api.Ctx, types.NamespacedName{
		Name:      common.WorkspaceRolebindingName(),
		Namespace: testNamespace,
	}, actualRB)
	assert.NoError(t, err, "Rolebinding should be created")
	assert.Equal(t, common.WorkspaceRoleName(), actualRB.RoleRef.Name, "Rolebinding shold reference default role")
	expectedSAName := common.ServiceAccountName(testdw)
	assert.True(t, testHasSubject(expectedSAName, testNamespace, actualRB), "Created rolebinding should have both workspace SAs as subjects")
	expectedSAName2 := common.ServiceAccountName(testdw2)
	assert.True(t, testHasSubject(expectedSAName2, testNamespace, actualRB), "Created rolebinding should have both workspace SAs as subjects")
}

func TestCreatesSCCRolebindingIfNotExists(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	testdw := getTestDevWorkspaceWithAttributes(t, "test-devworkspace", constants.WorkspaceSCCAttribute, testSCCName)
	api := getTestClusterAPI(t, testdw.DevWorkspace)
	retryErr := &dwerrors.RetryError{}
	err := syncRolebindings(testdw, api)
	if assert.Error(t, err, "Should return RetryError to indicate that default rolebinding was created") {
		assert.ErrorAs(t, err, &retryErr, "Error should have RetryError type")
	}
	err = syncRolebindings(testdw, api)
	if assert.Error(t, err, "Should return RetryError to indicate that SCC rolebinding was created") {
		assert.ErrorAs(t, err, &retryErr, "Error should have RetryError type")
	}
	err = syncRolebindings(testdw, api)
	assert.NoError(t, err, "Should not return error if rolebindings are in sync")
	actualRB := &rbacv1.RoleBinding{}
	err = api.Client.Get(api.Ctx, types.NamespacedName{
		Name:      common.WorkspaceSCCRolebindingName(testSCCName),
		Namespace: testNamespace,
	}, actualRB)
	assert.NoError(t, err, "Rolebinding should be created")
	assert.Equal(t, common.WorkspaceSCCRoleName(testSCCName), actualRB.RoleRef.Name, "Rolebinding shold reference default role")
	expectedSAName := common.ServiceAccountName(testdw)
	assert.True(t, testHasSubject(expectedSAName, testNamespace, actualRB), "Created rolebinding should have workspace SA as subject")
}

func TestAddsMultipleSubjectsToSCCRolebinding(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	testdw := getTestDevWorkspaceWithAttributes(t, "test-devworkspace", constants.WorkspaceSCCAttribute, testSCCName)
	testdw2 := getTestDevWorkspaceWithAttributes(t, "test-devworkspace-2", constants.WorkspaceSCCAttribute, testSCCName)
	api := getTestClusterAPI(t, testdw.DevWorkspace)
	retryErr := &dwerrors.RetryError{}
	err := syncRolebindings(testdw, api)
	if assert.Error(t, err, "Should return RetryError to indicate that default rolebinding was created") {
		assert.ErrorAs(t, err, &retryErr, "Error should have RetryError type")
	}
	err = syncRolebindings(testdw, api)
	if assert.Error(t, err, "Should return RetryError to indicate that SCC rolebinding was created") {
		assert.ErrorAs(t, err, &retryErr, "Error should have RetryError type")
	}
	err = syncRolebindings(testdw, api)
	assert.NoError(t, err, "Should not return error if rolebindings are in sync")
	err = syncRolebindings(testdw2, api)
	if assert.Error(t, err, "Should return RetryError to indicate that default rolebinding was created") {
		assert.ErrorAs(t, err, &retryErr, "Error should have RetryError type")
	}
	err = syncRolebindings(testdw2, api)
	if assert.Error(t, err, "Should return RetryError to indicate that SCC rolebinding was created") {
		assert.ErrorAs(t, err, &retryErr, "Error should have RetryError type")
	}
	err = syncRolebindings(testdw2, api)
	assert.NoError(t, err, "Should not return error if rolebindings are in sync")

	actualRB := &rbacv1.RoleBinding{}
	err = api.Client.Get(api.Ctx, types.NamespacedName{
		Name:      common.WorkspaceSCCRolebindingName(testSCCName),
		Namespace: testNamespace,
	}, actualRB)
	assert.NoError(t, err, "Rolebinding should be created")
	assert.Equal(t, common.WorkspaceSCCRoleName(testSCCName), actualRB.RoleRef.Name, "Rolebinding shold reference default role")
	expectedSAName := common.ServiceAccountName(testdw)
	assert.True(t, testHasSubject(expectedSAName, testNamespace, actualRB), "Created SCC rolebinding should have both workspace SAs as subjects")
	expectedSAName2 := common.ServiceAccountName(testdw2)
	assert.True(t, testHasSubject(expectedSAName2, testNamespace, actualRB), "Created SCC rolebinding should have both workspace SAs as subjects")
}

func testHasSubject(subjName, namespace string, rolebinding *rbacv1.RoleBinding) bool {
	for _, subject := range rolebinding.Subjects {
		if subject.Name == subjName && subject.Namespace == namespace && subject.Kind == rbacv1.ServiceAccountKind {
			return true
		}
	}
	return false
}
