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

func TestValidateEndpoints(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = dwv2.AddToScheme(scheme)

	t.Run("Conflict in same namespace", func(t *testing.T) {
		// Workspace with a discoverable endpoint in namespace "test-namespace"
		workspace := &dwv2.DevWorkspace{}
		err := loadObjectFromFile("workspace-1", workspace, "test-devworkspace.yaml")
		assert.NoError(t, err, "Failed to load test workspace")
		workspace.SetUID(types.UID("uid-1"))
		workspace.SetNamespace("test-namespace")

		// Another workspace with a conflicting discoverable endpoint in the SAME namespace
		otherWorkspaceSameNS := &dwv2.DevWorkspace{}
		err = loadObjectFromFile("workspace-2", otherWorkspaceSameNS, "test-devworkspace.yaml")
		assert.NoError(t, err, "Failed to load other test workspace")
		otherWorkspaceSameNS.SetUID(types.UID("uid-2"))
		otherWorkspaceSameNS.SetNamespace("test-namespace")

		// Test for conflict in same namespace
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(otherWorkspaceSameNS).Build()
		handler := &WebhookHandler{Client: fakeClient}
		err = handler.validateEndpoints(context.TODO(), workspace)
		assert.Error(t, err, "Expected a conflict error for workspaces in the same namespace")
		assert.Contains(t, err.Error(), "already in use by workspace")
		assert.Contains(t, err.Error(), "test-endpoint")
	})

	t.Run("No conflict in different namespace", func(t *testing.T) {
		// Workspace in "test-namespace"
		workspace := &dwv2.DevWorkspace{}
		err := loadObjectFromFile("workspace-1", workspace, "test-devworkspace.yaml")
		assert.NoError(t, err, "Failed to load test workspace")
		workspace.SetUID(types.UID("uid-1"))
		workspace.SetNamespace("test-namespace")

		// Another workspace with the same endpoint name but in a DIFFERENT namespace
		otherWorkspaceDiffNS := &dwv2.DevWorkspace{}
		err = loadObjectFromFile("workspace-3", otherWorkspaceDiffNS, "test-devworkspace.yaml")
		assert.NoError(t, err, "Failed to load third test workspace")
		otherWorkspaceDiffNS.SetUID(types.UID("uid-3"))
		otherWorkspaceDiffNS.SetNamespace("other-namespace")

		// Test no conflict in different namespace (workspace only queries its own namespace)
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(otherWorkspaceDiffNS).Build()
		handler := &WebhookHandler{Client: fakeClient}
		err = handler.validateEndpoints(context.TODO(), workspace)
		assert.NoError(t, err, "Did not expect an error for workspaces in different namespaces")
	})

	t.Run("No conflict when endpoint name is different", func(t *testing.T) {
		workspace := &dwv2.DevWorkspace{}
		err := loadObjectFromFile("workspace-1", workspace, "test-devworkspace.yaml")
		assert.NoError(t, err, "Failed to load test workspace")
		workspace.SetUID(types.UID("uid-1"))
		workspace.SetNamespace("test-namespace")
		workspace.Spec.Template.Components[0].Container.Endpoints[0].Name = "new-endpoint"

		otherWorkspace := &dwv2.DevWorkspace{}
		err = loadObjectFromFile("workspace-2", otherWorkspace, "test-devworkspace.yaml")
		assert.NoError(t, err, "Failed to load other test workspace")
		otherWorkspace.SetUID(types.UID("uid-2"))
		otherWorkspace.SetNamespace("test-namespace")

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(otherWorkspace).Build()
		handler := &WebhookHandler{Client: fakeClient}
		err = handler.validateEndpoints(context.TODO(), workspace)
		assert.NoError(t, err, "Did not expect an error for different endpoint names")
	})

	t.Run("Conflict detected even when workspace is being deleted", func(t *testing.T) {
		workspace := &dwv2.DevWorkspace{}
		err := loadObjectFromFile("workspace-1", workspace, "test-devworkspace.yaml")
		assert.NoError(t, err, "Failed to load test workspace")
		workspace.SetUID(types.UID("uid-1"))
		workspace.SetNamespace("test-namespace")

		// Workspace being deleted with same endpoint name
		deletingWorkspace := &dwv2.DevWorkspace{}
		err = loadObjectFromFile("workspace-deleting", deletingWorkspace, "test-devworkspace.yaml")
		assert.NoError(t, err, "Failed to load deleting workspace")
		deletingWorkspace.SetUID(types.UID("uid-deleting"))
		deletingWorkspace.SetNamespace("test-namespace")
		now := metav1.Now()
		deletingWorkspace.DeletionTimestamp = &now
		// Add finalizer - required by fake client when setting deletionTimestamp
		deletingWorkspace.Finalizers = []string{"test-finalizer"}

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(deletingWorkspace).Build()
		handler := &WebhookHandler{Client: fakeClient}
		err = handler.validateEndpoints(context.TODO(), workspace)
		assert.Error(t, err, "Should detect conflict even with workspace being deleted")
		assert.Contains(t, err.Error(), "workspace-deleting")
	})

	t.Run("No conflict when workspace has no discoverable endpoints", func(t *testing.T) {
		workspace := &dwv2.DevWorkspace{}
		err := loadObjectFromFile("workspace-1", workspace, "test-devworkspace.yaml")
		assert.NoError(t, err, "Failed to load test workspace")
		workspace.SetUID(types.UID("uid-1"))
		workspace.SetNamespace("test-namespace")
		// Remove discoverable attribute
		workspace.Spec.Template.Components[0].Container.Endpoints[0].Attributes = nil

		otherWorkspace := &dwv2.DevWorkspace{}
		err = loadObjectFromFile("workspace-2", otherWorkspace, "test-devworkspace.yaml")
		assert.NoError(t, err, "Failed to load other test workspace")
		otherWorkspace.SetUID(types.UID("uid-2"))
		otherWorkspace.SetNamespace("test-namespace")

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(otherWorkspace).Build()
		handler := &WebhookHandler{Client: fakeClient}
		err = handler.validateEndpoints(context.TODO(), workspace)
		assert.NoError(t, err, "Did not expect an error when workspace has no discoverable endpoints")
	})

	t.Run("Multiple workspaces in different namespaces can have same endpoint", func(t *testing.T) {
		// Workspace 1 in namespace-a
		workspace1 := &dwv2.DevWorkspace{}
		err := loadObjectFromFile("workspace-ns-a", workspace1, "test-devworkspace.yaml")
		assert.NoError(t, err, "Failed to load workspace 1")
		workspace1.SetUID(types.UID("uid-ns-a"))
		workspace1.SetNamespace("namespace-a")

		// Workspace 2 in namespace-b (will be in the fake client as existing)
		workspace2 := &dwv2.DevWorkspace{}
		err = loadObjectFromFile("workspace-ns-b", workspace2, "test-devworkspace.yaml")
		assert.NoError(t, err, "Failed to load workspace 2")
		workspace2.SetUID(types.UID("uid-ns-b"))
		workspace2.SetNamespace("namespace-b")

		// Workspace 3 in namespace-c (will be in the fake client as existing)
		workspace3 := &dwv2.DevWorkspace{}
		err = loadObjectFromFile("workspace-ns-c", workspace3, "test-devworkspace.yaml")
		assert.NoError(t, err, "Failed to load workspace 3")
		workspace3.SetUID(types.UID("uid-ns-c"))
		workspace3.SetNamespace("namespace-c")

		// All three workspaces exist, but in different namespaces
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).
			WithObjects(workspace2, workspace3).Build()
		handler := &WebhookHandler{Client: fakeClient}

		// Validating workspace1 should succeed (different namespaces)
		err = handler.validateEndpoints(context.TODO(), workspace1)
		assert.NoError(t, err, "Should allow same endpoint name in different namespaces")
	})
}
