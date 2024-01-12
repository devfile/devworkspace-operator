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
	"testing"
	"time"

	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestDeletesRoleAndRolebindingWhenLastWorkspaceIsDeleted(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	testdw := getTestDevWorkspace("test-devworkspace")
	testdw.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	api := getTestClusterAPI(t, testdw.DevWorkspace, newRole, newRolebinding)
	retryErr := &dwerrors.RetryError{}
	err := FinalizeRBAC(testdw, api)
	if assert.Error(t, err, "Should return error to indicate role deleted") {
		assert.ErrorAs(t, err, &retryErr, "Error should be RetryError")
		assert.Regexp(t, fmt.Sprintf("deleted role .* in namespace %s", testNamespace), err.Error())
	}
	err = FinalizeRBAC(testdw, api)
	if assert.Error(t, err, "Should return error to indicate rolebinding deleted") {
		assert.ErrorAs(t, err, &retryErr, "Error should be RetryError")
		assert.Regexp(t, fmt.Sprintf("deleted rolebinding .* in namespace %s", testNamespace), err.Error())
	}
	err = FinalizeRBAC(testdw, api)
	assert.NoError(t, err, "Should not return error once role and rolebinding deleted")
}

func TestDeletesRoleAndRolebindingWhenAllWorkspacesAreDeleted(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	testdw := getTestDevWorkspace("test-devworkspace")
	testdw2 := getTestDevWorkspace("test-devworkspace2")
	testdw.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	testdw2.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	api := getTestClusterAPI(t, testdw.DevWorkspace, testdw2.DevWorkspace, newRole, newRolebinding)

	retryErr := &dwerrors.RetryError{}
	err := FinalizeRBAC(testdw, api)
	if assert.Error(t, err, "Should return error to indicate role deleted") {
		assert.ErrorAs(t, err, &retryErr, "Error should be RetryError")
		assert.Regexp(t, fmt.Sprintf("deleted role .* in namespace %s", testNamespace), err.Error())
	}
	err = FinalizeRBAC(testdw, api)
	if assert.Error(t, err, "Should return error to indicate rolebinding deleted") {
		assert.ErrorAs(t, err, &retryErr, "Error should be RetryError")
		assert.Regexp(t, fmt.Sprintf("deleted rolebinding .* in namespace %s", testNamespace), err.Error())
	}
	err = FinalizeRBAC(testdw, api)
	assert.NoError(t, err, "Should not return error once role and rolebinding deleted")
}

func TestShouldRemoveWorkspaceSAFromRolebindingWhenDeleted(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	testdw := getTestDevWorkspace("test-devworkspace")
	testdw2 := getTestDevWorkspace("test-devworkspace2")
	testdw.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	testdwSAName := common.ServiceAccountName(testdw)
	testdw2SAName := common.ServiceAccountName(testdw2)
	testrb := newRolebinding.DeepCopy()
	testrb.Subjects = append(testrb.Subjects,
		rbacv1.Subject{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      testdwSAName,
			Namespace: testNamespace,
		}, rbacv1.Subject{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      testdw2SAName,
			Namespace: testNamespace,
		})
	api := getTestClusterAPI(t, testdw.DevWorkspace, testdw2.DevWorkspace, testrb, newRole)
	retryErr := &dwerrors.RetryError{}
	err := FinalizeRBAC(testdw, api)
	if assert.Error(t, err, "Should return error to indicate rolebinding updated") {
		assert.ErrorAs(t, err, &retryErr, "Error should be RetryError")
	}
	err = FinalizeRBAC(testdw, api)
	assert.NoError(t, err, "Should not return error once rolebinding is in sync")

	actualRolebinding := &rbacv1.RoleBinding{}
	err = api.Client.Get(api.Ctx, types.NamespacedName{
		Name:      common.WorkspaceRolebindingName(),
		Namespace: testNamespace,
	}, actualRolebinding)
	assert.NoError(t, err, "Unexpected test error getting rolebinding")
	assert.False(t, testHasSubject(testdwSAName, testNamespace, actualRolebinding), "Should remove delete workspace SA from rolebinding subjects")
	assert.True(t, testHasSubject(testdw2SAName, testNamespace, actualRolebinding), "Should leave workspace SA in rolebinding subjects")
}

func TestFinalizeDoesNothingWhenRolebindingDoesNotExist(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	testdw := getTestDevWorkspace("test-devworkspace")
	testdw2 := getTestDevWorkspace("test-devworkspace2")
	testdw.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	api := getTestClusterAPI(t, testdw.DevWorkspace, testdw2.DevWorkspace, newRole)
	err := FinalizeRBAC(testdw, api)
	assert.NoError(t, err, "Should not return error once rolebinding is in sync")

	actualRolebinding := &rbacv1.RoleBinding{}
	err = api.Client.Get(api.Ctx, types.NamespacedName{
		Name:      common.WorkspaceRolebindingName(),
		Namespace: testNamespace,
	}, actualRolebinding)
	if assert.Error(t, err, "Expect error when getting non-existent rolebinding") {
		assert.True(t, k8sErrors.IsNotFound(err), "Error should have IsNotFound type")
	}
}

func TestDeletesSCCRoleAndRolebindingWhenLastWorkspaceIsDeleted(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	testdw := getTestDevWorkspaceWithAttributes(t, "test-devworkspace", constants.WorkspaceSCCAttribute, testSCCName)
	testdw2 := getTestDevWorkspace("test-devworkspace2")
	testdw.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	api := getTestClusterAPI(t, testdw.DevWorkspace, testdw2.DevWorkspace, newSCCRole, newSCCRolebinding)
	retryErr := &dwerrors.RetryError{}
	err := FinalizeRBAC(testdw, api)
	if assert.Error(t, err, "Should return error to indicate role deleted") {
		assert.ErrorAs(t, err, &retryErr, "Error should be RetryError")
		assert.Regexp(t, fmt.Sprintf("deleted role .* in namespace %s", testNamespace), err.Error())
	}
	err = FinalizeRBAC(testdw, api)
	if assert.Error(t, err, "Should return error to indicate rolebinding deleted") {
		assert.ErrorAs(t, err, &retryErr, "Error should be RetryError")
		assert.Regexp(t, fmt.Sprintf("deleted rolebinding .* in namespace %s", testNamespace), err.Error())
	}
	err = FinalizeRBAC(testdw, api)
	assert.NoError(t, err, "Should not return error once role and rolebinding deleted")
}

func TestDeletesSCCRoleAndRolebindingWhenAllWorkspacesAreDeleted(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	testdw := getTestDevWorkspaceWithAttributes(t, "test-devworkspace", constants.WorkspaceSCCAttribute, testSCCName)
	testdw2 := getTestDevWorkspaceWithAttributes(t, "test-devworkspace2", constants.WorkspaceSCCAttribute, testSCCName)
	testdw.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	testdw2.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	api := getTestClusterAPI(t, testdw.DevWorkspace, testdw2.DevWorkspace, newSCCRole, newSCCRolebinding)

	retryErr := &dwerrors.RetryError{}
	err := FinalizeRBAC(testdw, api)
	if assert.Error(t, err, "Should return error to indicate role deleted") {
		assert.ErrorAs(t, err, &retryErr, "Error should be RetryError")
		assert.Regexp(t, fmt.Sprintf("deleted role .* in namespace %s", testNamespace), err.Error())
	}
	err = FinalizeRBAC(testdw, api)
	if assert.Error(t, err, "Should return error to indicate rolebinding deleted") {
		assert.ErrorAs(t, err, &retryErr, "Error should be RetryError")
		assert.Regexp(t, fmt.Sprintf("deleted rolebinding .* in namespace %s", testNamespace), err.Error())
	}
	err = FinalizeRBAC(testdw, api)
	assert.NoError(t, err, "Should not return error once role and rolebinding deleted")
}

func TestShouldRemoveWorkspaceSAFromSCCRolebindingWhenDeleted(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	testdw := getTestDevWorkspaceWithAttributes(t, "test-devworkspace", constants.WorkspaceSCCAttribute, testSCCName)
	testdw2 := getTestDevWorkspaceWithAttributes(t, "test-devworkspace2", constants.WorkspaceSCCAttribute, testSCCName)
	testdw.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	testdwSAName := common.ServiceAccountName(testdw)
	testdw2SAName := common.ServiceAccountName(testdw2)
	testrb := newSCCRolebinding.DeepCopy()
	testrb.Subjects = append(testrb.Subjects,
		rbacv1.Subject{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      testdwSAName,
			Namespace: testNamespace,
		}, rbacv1.Subject{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      testdw2SAName,
			Namespace: testNamespace,
		})
	api := getTestClusterAPI(t, testdw.DevWorkspace, testdw2.DevWorkspace, testrb, newSCCRole)
	retryErr := &dwerrors.RetryError{}
	err := FinalizeRBAC(testdw, api)
	if assert.Error(t, err, "Should return error to indicate rolebinding updated") {
		assert.ErrorAs(t, err, &retryErr, "Error should be RetryError")
	}
	err = FinalizeRBAC(testdw, api)
	assert.NoError(t, err, "Should not return error once rolebinding is in sync")

	actualRolebinding := &rbacv1.RoleBinding{}
	err = api.Client.Get(api.Ctx, types.NamespacedName{
		Name:      common.WorkspaceSCCRolebindingName(testSCCName),
		Namespace: testNamespace,
	}, actualRolebinding)
	assert.NoError(t, err, "Unexpected test error getting rolebinding")
	assert.False(t, testHasSubject(testdwSAName, testNamespace, actualRolebinding), "Should remove delete workspace SA from rolebinding subjects")
	assert.True(t, testHasSubject(testdw2SAName, testNamespace, actualRolebinding), "Should leave workspace SA in rolebinding subjects")
}

func TestFinalizeDoesNothingWhenSCCRolebindingDoesNotExist(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	testdw := getTestDevWorkspaceWithAttributes(t, "test-devworkspace", constants.WorkspaceSCCAttribute, testSCCName)
	testdw2 := getTestDevWorkspaceWithAttributes(t, "test-devworkspace2", constants.WorkspaceSCCAttribute, testSCCName)
	testdw.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	api := getTestClusterAPI(t, testdw.DevWorkspace, testdw2.DevWorkspace, newSCCRole)
	err := FinalizeRBAC(testdw, api)
	assert.NoError(t, err, "Should not return error once rolebinding is in sync")

	actualRolebinding := &rbacv1.RoleBinding{}
	err = api.Client.Get(api.Ctx, types.NamespacedName{
		Name:      common.WorkspaceSCCRolebindingName(testSCCName),
		Namespace: testNamespace,
	}, actualRolebinding)
	if assert.Error(t, err, "Expect error when getting non-existent rolebinding") {
		assert.True(t, k8sErrors.IsNotFound(err), "Error should have IsNotFound type")
	}
}
