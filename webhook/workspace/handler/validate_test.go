// Copyright (c) 2019-2025 Red Hat, Inc.
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

package handler

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	dwv2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/controllers/controller/devworkspacerouting/solvers"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

func loadObjectFromFile(objName string, obj client.Object, filename string) error {
	path := filepath.Join("testdata", filename)
	bytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(bytes, obj)
	if err != nil {
		return err
	}
	obj.SetName(objName)
	return nil
}

func setupWorkspace(t *testing.T, name, uid, namespace string) *dwv2.DevWorkspace {
	workspace := &dwv2.DevWorkspace{}
	err := loadObjectFromFile(name, workspace, "test-devworkspace.yaml")
	assert.NoError(t, err, "Failed to load workspace")
	workspace.SetUID(types.UID(uid))
	workspace.SetNamespace(namespace)
	return workspace
}

func TestValidateEndpoints(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = dwv2.AddToScheme(scheme)

	t.Run("Conflict in same namespace", func(t *testing.T) {
		// Workspace with a discoverable endpoint in namespace "test-namespace"
		workspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")

		// Another workspace with a conflicting discoverable endpoint in the SAME namespace
		otherWorkspaceSameNS := setupWorkspace(t, "workspace-2", "uid-2", "test-namespace")

		// Test for conflict in same namespace
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(otherWorkspaceSameNS).Build()
		handler := &WebhookHandler{Client: fakeClient}
		err := handler.validateEndpoints(context.TODO(), workspace)
		assert.Error(t, err, "Expected a conflict error for workspaces in the same namespace")

		var conflictErr *solvers.ServiceConflictError
		assert.ErrorAs(t, err, &conflictErr, "Error should be a ServiceConflictError")
		assert.Equal(t, "test-endpoint", conflictErr.EndpointName, "Conflict should be on 'test-endpoint'")
		assert.Equal(t, "workspace-2", conflictErr.WorkspaceName, "Conflict should reference 'workspace-2'")
	})

	t.Run("No conflict in different namespace", func(t *testing.T) {
		// Workspace in "test-namespace"
		workspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")

		// Another workspace with the same endpoint name but in a DIFFERENT namespace
		otherWorkspaceDiffNS := setupWorkspace(t, "workspace-3", "uid-3", "other-namespace")

		// Test no conflict in different namespace (workspace only queries its own namespace)
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(otherWorkspaceDiffNS).Build()
		handler := &WebhookHandler{Client: fakeClient}
		err := handler.validateEndpoints(context.TODO(), workspace)
		assert.NoError(t, err, "Did not expect an error for workspaces in different namespaces")
	})

	t.Run("No conflict when endpoint name is different", func(t *testing.T) {
		workspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
		workspace.Spec.Template.Components[0].Container.Endpoints[0].Name = "new-endpoint"

		otherWorkspace := setupWorkspace(t, "workspace-2", "uid-2", "test-namespace")

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(otherWorkspace).Build()
		handler := &WebhookHandler{Client: fakeClient}
		err := handler.validateEndpoints(context.TODO(), workspace)
		assert.NoError(t, err, "Did not expect an error for different endpoint names")
	})

	t.Run("Conflict detected even when workspace is being deleted", func(t *testing.T) {
		workspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")

		// Workspace being deleted with same endpoint name
		deletingWorkspace := setupWorkspace(t, "workspace-deleting", "uid-deleting", "test-namespace")
		now := metav1.Now()
		deletingWorkspace.DeletionTimestamp = &now
		// Add finalizer - required by fake client when setting deletionTimestamp
		deletingWorkspace.Finalizers = []string{"test-finalizer"}

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(deletingWorkspace).Build()
		handler := &WebhookHandler{Client: fakeClient}
		err := handler.validateEndpoints(context.TODO(), workspace)
		assert.Error(t, err, "Should detect conflict even with workspace being deleted")

		var conflictErr *solvers.ServiceConflictError
		assert.ErrorAs(t, err, &conflictErr, "Error should be a ServiceConflictError")
		assert.Equal(t, "test-endpoint", conflictErr.EndpointName, "Conflict should be on 'test-endpoint'")
		assert.Equal(t, "workspace-deleting", conflictErr.WorkspaceName, "Conflict should reference 'workspace-deleting'")
	})

	t.Run("No conflict when workspace has no discoverable endpoints", func(t *testing.T) {
		workspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
		// Remove discoverable attribute
		workspace.Spec.Template.Components[0].Container.Endpoints[0].Attributes = nil

		otherWorkspace := setupWorkspace(t, "workspace-2", "uid-2", "test-namespace")

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(otherWorkspace).Build()
		handler := &WebhookHandler{Client: fakeClient}
		err := handler.validateEndpoints(context.TODO(), workspace)
		assert.NoError(t, err, "Did not expect an error when workspace has no discoverable endpoints")
	})

	t.Run("Multiple workspaces in different namespaces can have same endpoint", func(t *testing.T) {
		// Workspace 1 in namespace-a
		workspace1 := setupWorkspace(t, "workspace-ns-a", "uid-ns-a", "namespace-a")

		// Workspace 2 in namespace-b (will be in the fake client as existing)
		workspace2 := setupWorkspace(t, "workspace-ns-b", "uid-ns-b", "namespace-b")

		// Workspace 3 in namespace-c (will be in the fake client as existing)
		workspace3 := setupWorkspace(t, "workspace-ns-c", "uid-ns-c", "namespace-c")

		// All three workspaces exist, but in different namespaces
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).
			WithObjects(workspace2, workspace3).Build()
		handler := &WebhookHandler{Client: fakeClient}

		// Validating workspace1 should succeed (different namespaces)
		err := handler.validateEndpoints(context.TODO(), workspace1)
		assert.NoError(t, err, "Should allow same endpoint name in different namespaces")
	})
}
