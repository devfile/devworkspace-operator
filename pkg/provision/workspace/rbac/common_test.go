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
	"context"
	"fmt"
	"testing"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testNamespace = "test-namespace"
	testSCCName   = "test-scc"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(dw.AddToScheme(scheme))
}

var (
	oldRole = &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.OldWorkspaceRoleName(),
			Namespace: testNamespace,
		},
		Rules: []rbacv1.PolicyRule{},
	}
	oldRolebinding = &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.OldWorkspaceRolebindingName(),
			Namespace: testNamespace,
		},
	}
	newRole = &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.WorkspaceRoleName(),
			Namespace: testNamespace,
		},
		Rules: []rbacv1.PolicyRule{},
	}
	newRolebinding = &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.WorkspaceRolebindingName(),
			Namespace: testNamespace,
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "Role",
			Name: common.WorkspaceRoleName(),
		},
	}
	newSCCRole = &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.WorkspaceSCCRoleName(testSCCName),
			Namespace: testNamespace,
		},
		Rules: []rbacv1.PolicyRule{},
	}
	newSCCRolebinding = &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.WorkspaceSCCRolebindingName(testSCCName),
			Namespace: testNamespace,
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "Role",
			Name: common.WorkspaceSCCRoleName(testSCCName),
		},
	}
)

func TestSyncRBAC(t *testing.T) {
	testdw1 := getTestDevWorkspaceWithAttributes(t, "test-devworkspace", constants.WorkspaceSCCAttribute, testSCCName)
	testdw2 := getTestDevWorkspaceWithAttributes(t, "test-devworkspace2", constants.WorkspaceSCCAttribute, testSCCName)
	testdw1SAName := common.ServiceAccountName(testdw1)
	testdw2SAName := common.ServiceAccountName(testdw2)
	api := getTestClusterAPI(t, testdw1.DevWorkspace, testdw2.DevWorkspace, oldRole, oldRolebinding)
	// Keep calling SyncRBAC until error returned is nil, to account for multiple steps
	iterCount := 0
	maxIters := 30
	retryErr := &dwerrors.RetryError{}
	for err := SyncRBAC(testdw1, api); err != nil; err = SyncRBAC(testdw1, api) {
		iterCount += 1
		if err == nil {
			break
		}
		if !assert.ErrorAs(t, err, &retryErr, "Unexpected error from SyncRBAC: %s", err) {
			return
		}
		if !assert.LessOrEqual(t, iterCount, maxIters, fmt.Sprintf("SyncRBAC did not sync everything within %d iterations", maxIters)) {
			return
		}
	}
	for err := SyncRBAC(testdw2, api); err != nil; err = SyncRBAC(testdw2, api) {
		iterCount += 1
		if err == nil {
			break
		}
		if !assert.ErrorAs(t, err, &retryErr, "Unexpected error from SyncRBAC: %s", err) {
			return
		}
		if !assert.LessOrEqual(t, iterCount, maxIters, fmt.Sprintf("SyncRBAC did not sync everything within %d iterations", maxIters)) {
			return
		}
	}
	actualRole := &rbacv1.Role{}
	err := api.Client.Get(api.Ctx, types.NamespacedName{
		Name:      common.WorkspaceRoleName(),
		Namespace: testNamespace,
	}, actualRole)
	assert.NoError(t, err, "Role should be created")

	actualSCCRole := &rbacv1.Role{}
	err = api.Client.Get(api.Ctx, types.NamespacedName{
		Name:      common.WorkspaceSCCRoleName(testSCCName),
		Namespace: testNamespace,
	}, actualSCCRole)
	assert.NoError(t, err, "SCC Role should be created")

	actualRolebinding := &rbacv1.RoleBinding{}
	err = api.Client.Get(api.Ctx, types.NamespacedName{
		Name:      common.WorkspaceRolebindingName(),
		Namespace: testNamespace,
	}, actualRolebinding)
	assert.NoError(t, err, "Role should be created")
	assert.True(t, testHasSubject(testdw1SAName, testNamespace, actualRolebinding), "Should have testdw1 SA as subject")
	assert.True(t, testHasSubject(testdw2SAName, testNamespace, actualRolebinding), "Should have testdw2 SA as subject")

	actualSCCRolebinding := &rbacv1.RoleBinding{}
	err = api.Client.Get(api.Ctx, types.NamespacedName{
		Name:      common.WorkspaceSCCRolebindingName(testSCCName),
		Namespace: testNamespace,
	}, actualSCCRolebinding)
	assert.NoError(t, err, "SCC Rolebindind should be created")
	assert.True(t, testHasSubject(testdw1SAName, testNamespace, actualSCCRolebinding), "Should have testdw1 SA as subject")
	assert.True(t, testHasSubject(testdw2SAName, testNamespace, actualSCCRolebinding), "Should have testdw2 SA as subject")
}

func getTestClusterAPI(t *testing.T, initialObjects ...client.Object) sync.ClusterAPI {
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initialObjects...).Build()
	return sync.ClusterAPI{
		Ctx:    context.Background(),
		Client: fakeClient,
		Scheme: scheme,
		Logger: testr.New(t),
	}
}

func getTestDevWorkspace(id string) *common.DevWorkspaceWithConfig {
	return &common.DevWorkspaceWithConfig{
		DevWorkspace: &dw.DevWorkspace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      id,
				Namespace: testNamespace,
			},

			Status: dw.DevWorkspaceStatus{
				DevWorkspaceId: id,
			},
		},
		Config: config.GetConfigForTesting(nil),
	}
}

func getTestDevWorkspaceWithAttributes(t *testing.T, id string, keysAndValues ...string) *common.DevWorkspaceWithConfig {
	attr := attributes.Attributes{}
	if len(keysAndValues)%2 != 0 {
		t.Fatalf("Invalid keysAndValues for getTestDevWorkspaceWithAttributes")
	}
	for i := 0; i < len(keysAndValues); i += 2 {
		attr.PutString(keysAndValues[i], keysAndValues[i+1])
	}
	return &common.DevWorkspaceWithConfig{
		Config: config.GetConfigForTesting(nil),
		DevWorkspace: &dw.DevWorkspace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      id,
				Namespace: testNamespace,
			},
			Spec: dw.DevWorkspaceSpec{
				Template: dw.DevWorkspaceTemplateSpec{
					DevWorkspaceTemplateSpecContent: dw.DevWorkspaceTemplateSpecContent{
						Attributes: attr,
					},
				},
			},
			Status: dw.DevWorkspaceStatus{
				DevWorkspaceId: id,
			},
		},
	}
}
