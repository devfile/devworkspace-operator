// Copyright (c) 2019-2026 Red Hat, Inc.
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
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	dwv2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"
)

func newRawExtension(t *testing.T, workspace *dwv2.DevWorkspace) runtime.RawExtension {
	bytes, err := json.Marshal(workspace)
	assert.NoError(t, err, "Failed to marshal workspace")
	return runtime.RawExtension{Raw: bytes}
}

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
		fakeClient := newIndexedWorkspaceClientBuilder(t, scheme).WithObjects(otherWorkspaceSameNS).Build()
		handler := &WebhookHandler{Client: fakeClient}
		conflict, err := handler.validateEndpoints(context.TODO(), workspace, discoverableEndpointNames(workspace))
		assert.NoError(t, err, "Did not expect an infrastructure error")
		assert.NotNil(t, conflict, "Expected a conflict for workspaces in the same namespace")
		assert.Equal(t, "test-endpoint", conflict.EndpointName, "Conflict should be on 'test-endpoint'")
		assert.Equal(t, "workspace-2", conflict.WorkspaceName, "Conflict should reference 'workspace-2'")
	})

	t.Run("No conflict in different namespace", func(t *testing.T) {
		// Workspace in "test-namespace"
		workspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")

		// Another workspace with the same endpoint name but in a DIFFERENT namespace
		otherWorkspaceDiffNS := setupWorkspace(t, "workspace-3", "uid-3", "other-namespace")

		// Test no conflict in different namespace (workspace only queries its own namespace)
		fakeClient := newIndexedWorkspaceClientBuilder(t, scheme).WithObjects(otherWorkspaceDiffNS).Build()
		handler := &WebhookHandler{Client: fakeClient}
		conflict, err := handler.validateEndpoints(context.TODO(), workspace, discoverableEndpointNames(workspace))
		assert.NoError(t, err)
		assert.Nil(t, conflict, "Did not expect a conflict for workspaces in different namespaces")
	})

	t.Run("No conflict when endpoint name is different", func(t *testing.T) {
		workspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
		workspace.Spec.Template.Components[0].Container.Endpoints[0].Name = "new-endpoint"

		otherWorkspace := setupWorkspace(t, "workspace-2", "uid-2", "test-namespace")

		fakeClient := newIndexedWorkspaceClientBuilder(t, scheme).WithObjects(otherWorkspace).Build()
		handler := &WebhookHandler{Client: fakeClient}
		conflict, err := handler.validateEndpoints(context.TODO(), workspace, discoverableEndpointNames(workspace))
		assert.NoError(t, err)
		assert.Nil(t, conflict, "Did not expect a conflict for different endpoint names")
	})

	t.Run("Conflict detected even when workspace is being deleted", func(t *testing.T) {
		workspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")

		// Workspace being deleted with same endpoint name
		deletingWorkspace := setupWorkspace(t, "workspace-deleting", "uid-deleting", "test-namespace")
		now := metav1.Now()
		deletingWorkspace.DeletionTimestamp = &now
		// Add finalizer - required by fake client when setting deletionTimestamp
		deletingWorkspace.Finalizers = []string{"test-finalizer"}

		fakeClient := newIndexedWorkspaceClientBuilder(t, scheme).WithObjects(deletingWorkspace).Build()
		handler := &WebhookHandler{Client: fakeClient}
		conflict, err := handler.validateEndpoints(context.TODO(), workspace, discoverableEndpointNames(workspace))
		assert.NoError(t, err, "Did not expect an infrastructure error")
		assert.NotNil(t, conflict, "Should detect conflict even with workspace being deleted")
		assert.Equal(t, "test-endpoint", conflict.EndpointName, "Conflict should be on 'test-endpoint'")
		assert.Equal(t, "workspace-deleting", conflict.WorkspaceName, "Conflict should reference 'workspace-deleting'")
	})

	t.Run("No conflict when workspace has no discoverable endpoints", func(t *testing.T) {
		workspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
		// Remove discoverable attribute
		workspace.Spec.Template.Components[0].Container.Endpoints[0].Attributes = nil

		otherWorkspace := setupWorkspace(t, "workspace-2", "uid-2", "test-namespace")

		fakeClient := newIndexedWorkspaceClientBuilder(t, scheme).WithObjects(otherWorkspace).Build()
		handler := &WebhookHandler{Client: fakeClient}
		conflict, err := handler.validateEndpoints(context.TODO(), workspace, discoverableEndpointNames(workspace))
		assert.NoError(t, err)
		assert.Nil(t, conflict, "Did not expect a conflict when workspace has no discoverable endpoints")
	})

	t.Run("No conflict when other workspace endpoint is not discoverable", func(t *testing.T) {
		// Current workspace has a discoverable endpoint
		workspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")

		// Other workspace has endpoint with same name but NOT discoverable
		otherWorkspace := setupWorkspace(t, "workspace-2", "uid-2", "test-namespace")
		otherWorkspace.Spec.Template.Components[0].Container.Endpoints[0].Attributes = nil

		fakeClient := newIndexedWorkspaceClientBuilder(t, scheme).WithObjects(otherWorkspace).Build()
		handler := &WebhookHandler{Client: fakeClient}
		conflict, err := handler.validateEndpoints(context.TODO(), workspace, discoverableEndpointNames(workspace))
		assert.NoError(t, err)
		assert.Nil(t, conflict, "Should not conflict when other workspace's endpoint is not discoverable")
	})

	t.Run("Ignores non-discoverable aliases before a discoverable conflict", func(t *testing.T) {
		workspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
		workspace.Spec.Template.Components[0].Container.Endpoints[0].Name = "my-endpoint"
		otherWorkspace := setupWorkspace(t, "workspace-2", "uid-2", "test-namespace")
		endpoint := otherWorkspace.Spec.Template.Components[0].Container.Endpoints[0]
		otherWorkspace.Spec.Template.Components[0].Container.Endpoints = []dwv2.Endpoint{
			{Name: "my---endpoint", TargetPort: 8081, Attributes: nil},
			{Name: "my----endpoint", TargetPort: 8082, Attributes: attributes.Attributes{}},
			{Name: "my-----endpoint", TargetPort: 8083, Attributes: attributes.Attributes{}.PutBoolean("discoverable", false)},
		}
		fakeClient := newIndexedWorkspaceClientBuilder(t, scheme).WithObjects(otherWorkspace).Build()
		handler := &WebhookHandler{Client: fakeClient}
		conflict, err := handler.validateEndpoints(context.TODO(), workspace, discoverableEndpointNames(workspace))
		assert.NoError(t, err)
		assert.Nil(t, conflict, "Sanitized names alone must not make non-discoverable endpoints conflict")

		endpoint.Name = "my--endpoint"
		otherWorkspace.Spec.Template.Components[0].Container.Endpoints = append(otherWorkspace.Spec.Template.Components[0].Container.Endpoints, endpoint)
		assert.NoError(t, fakeClient.Update(context.TODO(), otherWorkspace))
		conflict, err = handler.validateEndpoints(context.TODO(), workspace, discoverableEndpointNames(workspace))
		assert.NoError(t, err)
		if assert.NotNil(t, conflict, "Must continue scanning past non-discoverable aliases") {
			assert.Equal(t, "my--endpoint", conflict.EndpointName, "Report the conflicting raw endpoint name")
			assert.Equal(t, "workspace-2", conflict.WorkspaceName)
		}
	})

	t.Run("Conflict detected when endpoint names differ but sanitize to the same service name", func(t *testing.T) {
		workspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
		workspace.Spec.Template.Components[0].Container.Endpoints[0].Name = "my--endpoint"

		// Different raw name, but common.EndpointName collapses repeated hyphens in "my--endpoint" to "-".
		otherWorkspace := setupWorkspace(t, "workspace-2", "uid-2", "test-namespace")
		otherWorkspace.Spec.Template.Components[0].Container.Endpoints[0].Name = "my-endpoint"

		fakeClient := newIndexedWorkspaceClientBuilder(t, scheme).WithObjects(otherWorkspace).Build()
		handler := &WebhookHandler{Client: fakeClient}
		conflict, err := handler.validateEndpoints(context.TODO(), workspace, discoverableEndpointNames(workspace))
		assert.NoError(t, err, "Did not expect an infrastructure error")
		assert.NotNil(t, conflict, "Expected a conflict for endpoint names that sanitize to the same service name")
		assert.Equal(t, "my-endpoint", conflict.EndpointName, "Conflict should report the other workspace's raw endpoint name")
		assert.Equal(t, "workspace-2", conflict.WorkspaceName, "Conflict should reference 'workspace-2'")
	})

	t.Run("Ignores exposure none before an eligible sanitized alias", func(t *testing.T) {
		workspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
		workspace.Spec.Template.Components[0].Container.Endpoints[0].Name = "my-endpoint"
		otherWorkspace := setupWorkspace(t, "workspace-2", "uid-2", "test-namespace")
		ignored := otherWorkspace.Spec.Template.Components[0].Container.Endpoints[0]
		ignored.Name = "my---endpoint"
		ignored.Exposure = dwv2.NoneEndpointExposure
		eligible := ignored
		eligible.Name = "my--endpoint"
		eligible.Exposure = dwv2.InternalEndpointExposure
		eligible.TargetPort = 8081
		otherWorkspace.Spec.Template.Components[0].Container.Endpoints = []dwv2.Endpoint{ignored, eligible}

		fakeClient := newIndexedWorkspaceClientBuilder(t, scheme).WithObjects(otherWorkspace).Build()
		handler := &WebhookHandler{Client: fakeClient}
		conflict, err := handler.validateEndpoints(context.TODO(), workspace, discoverableEndpointNames(workspace))
		assert.NoError(t, err)
		if assert.NotNil(t, conflict, "Must continue scanning past exposure none") {
			assert.Equal(t, "my--endpoint", conflict.EndpointName, "Report the eligible raw endpoint name")
			assert.Equal(t, "workspace-2", conflict.WorkspaceName)
		}
	})

	t.Run("Multiple workspaces in different namespaces can have same endpoint", func(t *testing.T) {
		// Workspace 1 in namespace-a
		workspace1 := setupWorkspace(t, "workspace-ns-a", "uid-ns-a", "namespace-a")

		// Workspace 2 in namespace-b (will be in the fake client as existing)
		workspace2 := setupWorkspace(t, "workspace-ns-b", "uid-ns-b", "namespace-b")

		// Workspace 3 in namespace-c (will be in the fake client as existing)
		workspace3 := setupWorkspace(t, "workspace-ns-c", "uid-ns-c", "namespace-c")

		// All three workspaces exist, but in different namespaces
		fakeClient := newIndexedWorkspaceClientBuilder(t, scheme).
			WithObjects(workspace2, workspace3).Build()
		handler := &WebhookHandler{Client: fakeClient}

		// Validating workspace1 should succeed (different namespaces)
		conflict, err := handler.validateEndpoints(context.TODO(), workspace1, discoverableEndpointNames(workspace1))
		assert.NoError(t, err)
		assert.Nil(t, conflict, "Should allow same endpoint name in different namespaces")
	})
}

func TestValidateDevfilePeerDiscoverability(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, dwv2.AddToScheme(scheme))
	for _, name := range []string{"test--endpoint", "different-endpoint"} {
		for _, test := range []struct {
			name       string
			raw        string
			discovered bool
		}{
			{"missing", "", false},
			{"boolean true", "true", true},
			{"boolean false", "false", false},
			{"string true", `"true"`, true},
			{"string uppercase true", `"TRUE"`, true},
			{"string one", `"1"`, true},
			{"string false", `"false"`, false},
			{"string zero", `"0"`, false},
			{"malformed string", `"invalid"`, false},
			{"number", "1", false},
			{"null", "null", false},
			{"object", "{}", false},
		} {
			t.Run(name+"/"+test.name, func(t *testing.T) {
				workspace := setupWorkspace(t, "incoming", "incoming-uid", "test-namespace")
				peer := setupWorkspace(t, "peer", "peer-uid", "test-namespace")
				endpoint := &peer.Spec.Template.Components[0].Container.Endpoints[0]
				endpoint.Name = name
				endpoint.Attributes = nil
				if test.raw != "" {
					endpoint.Attributes = attributes.Attributes{"discoverable": apiextv1.JSON{Raw: []byte(test.raw)}}
				}
				handler := &WebhookHandler{
					Client:  newIndexedWorkspaceClientBuilder(t, scheme).WithObjects(peer).Build(),
					Decoder: admission.NewDecoder(scheme),
				}
				req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Create,
					Object:    newRawExtension(t, workspace),
				}}
				response := handler.ValidateDevfile(context.Background(), req)
				wantConflict := name == "test--endpoint" && test.discovered
				assert.Equal(t, !wantConflict, response.Allowed)
				if wantConflict {
					assert.Contains(t, response.Result.Message, name, "Report the raw peer name")
					assert.Contains(t, response.Result.Message, peer.Name)
				}
			})
		}
	}
}

func TestShouldCheckEndpointConflicts(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = dwv2.AddToScheme(scheme)
	handler := &WebhookHandler{Decoder: admission.NewDecoder(scheme)}

	t.Run("Skips decoding on update with no discoverable endpoints", func(t *testing.T) {
		workspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
		workspace.Spec.Template.Components[0].Container.Endpoints[0].Attributes = nil
		req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Update,
			OldObject: runtime.RawExtension{Raw: []byte("not-json")},
		}}
		handler := &WebhookHandler{}

		assert.NotPanics(t, func() {
			assert.False(t, handler.shouldCheckEndpointConflicts(req, discoverableEndpointNames(workspace)), "An empty incoming endpoint set needs no conflict check or old-object decoding")
		})
	})

	t.Run("Always checks on create", func(t *testing.T) {
		workspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
		req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			Object:    newRawExtension(t, workspace),
		}}
		assert.True(t, handler.shouldCheckEndpointConflicts(req, discoverableEndpointNames(workspace)), "Create requests must always be checked")
	})

	t.Run("Skips check on update when discoverable endpoints are unchanged", func(t *testing.T) {
		oldWorkspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
		newWorkspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
		newWorkspace.Spec.Started = !oldWorkspace.Spec.Started

		req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Update,
			Object:    newRawExtension(t, newWorkspace),
			OldObject: newRawExtension(t, oldWorkspace),
		}}
		assert.False(t, handler.shouldCheckEndpointConflicts(req, discoverableEndpointNames(newWorkspace)), "Unrelated update should skip the check")
	})

	t.Run("Checks on update when a discoverable endpoint is added", func(t *testing.T) {
		oldWorkspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
		newWorkspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
		newEndpoint := newWorkspace.Spec.Template.Components[0].Container.Endpoints[0]
		newEndpoint.Name = "another-endpoint"
		newWorkspace.Spec.Template.Components[0].Container.Endpoints = append(
			newWorkspace.Spec.Template.Components[0].Container.Endpoints, newEndpoint)

		req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Update,
			Object:    newRawExtension(t, newWorkspace),
			OldObject: newRawExtension(t, oldWorkspace),
		}}
		assert.True(t, handler.shouldCheckEndpointConflicts(req, discoverableEndpointNames(newWorkspace)), "Adding a discoverable endpoint must trigger the check")
	})

	t.Run("Skips check on update when a discoverable endpoint is only removed", func(t *testing.T) {
		oldWorkspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
		secondEndpoint := oldWorkspace.Spec.Template.Components[0].Container.Endpoints[0]
		secondEndpoint.Name = "second-endpoint"
		oldWorkspace.Spec.Template.Components[0].Container.Endpoints = append(
			oldWorkspace.Spec.Template.Components[0].Container.Endpoints, secondEndpoint)

		// newWorkspace keeps only the first endpoint - the second one was removed, nothing was added.
		newWorkspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")

		req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Update,
			Object:    newRawExtension(t, newWorkspace),
			OldObject: newRawExtension(t, oldWorkspace),
		}}
		assert.False(t, handler.shouldCheckEndpointConflicts(req, discoverableEndpointNames(newWorkspace)), "Removing a discoverable endpoint must not trigger the check")
	})

	t.Run("Skips check on update when a discoverable endpoint is renamed to the same sanitized service name", func(t *testing.T) {
		oldWorkspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
		oldWorkspace.Spec.Template.Components[0].Container.Endpoints[0].Name = "my--endpoint"
		newWorkspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
		// Collapsing repeated hyphens gives the same Service name, so the rename introduces no new conflict.
		newWorkspace.Spec.Template.Components[0].Container.Endpoints[0].Name = "my-endpoint"

		req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Update,
			Object:    newRawExtension(t, newWorkspace),
			OldObject: newRawExtension(t, oldWorkspace),
		}}
		assert.False(t, handler.shouldCheckEndpointConflicts(req, discoverableEndpointNames(newWorkspace)), "Renaming to a name that sanitizes to the same service name should skip the check")
	})

	t.Run("Checks on update when the old object cannot be decoded", func(t *testing.T) {
		workspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
		req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Update,
			Object:    newRawExtension(t, workspace),
			OldObject: runtime.RawExtension{Raw: []byte("not-json")},
		}}
		assert.True(t, handler.shouldCheckEndpointConflicts(req, discoverableEndpointNames(workspace)), "Decode failures must fail safe by running the check")
	})

	t.Run("Checks on update when a discoverable endpoint is renamed", func(t *testing.T) {
		oldWorkspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
		newWorkspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
		newWorkspace.Spec.Template.Components[0].Container.Endpoints[0].Name = "renamed-endpoint"

		req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Update,
			Object:    newRawExtension(t, newWorkspace),
			OldObject: newRawExtension(t, oldWorkspace),
		}}
		assert.True(t, handler.shouldCheckEndpointConflicts(req, discoverableEndpointNames(newWorkspace)), "Renaming a discoverable endpoint must trigger the check")
	})

	t.Run("Checks on update when an endpoint becomes discoverable", func(t *testing.T) {
		oldWorkspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
		oldWorkspace.Spec.Template.Components[0].Container.Endpoints[0].Attributes = nil
		newWorkspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")

		req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Update,
			Object:    newRawExtension(t, newWorkspace),
			OldObject: newRawExtension(t, oldWorkspace),
		}}
		assert.True(t, handler.shouldCheckEndpointConflicts(req, discoverableEndpointNames(newWorkspace)), "An endpoint becoming discoverable must trigger the check")
	})
}

func TestValidateDevfileEndpointConflictGating(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = dwv2.AddToScheme(scheme)

	t.Run("Denies invalid events with no discoverable endpoints and a malformed old object", func(t *testing.T) {
		workspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
		workspace.Spec.Template.Components[0].Container.Endpoints[0].Attributes = nil
		workspace.Spec.Template.Events = &dwv2.Events{DevWorkspaceEvents: dwv2.DevWorkspaceEvents{PreStart: []string{"missing-command"}}}
		handler := &WebhookHandler{Decoder: admission.NewDecoder(scheme)}
		req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Update,
			Object:    newRawExtension(t, workspace),
			OldObject: runtime.RawExtension{Raw: []byte("not-json")},
		}}

		resp := handler.ValidateDevfile(context.TODO(), req)
		assert.False(t, resp.Allowed, "Skipping endpoint checks must still validate events")
		assert.Contains(t, resp.Result.Message, "preStart type events are invalid")
		assert.Contains(t, resp.Result.Message, "missing-command does not map to a valid devfile command")
	})

	t.Run("Allows an unrelated update even though another workspace already has a conflicting endpoint", func(t *testing.T) {
		otherWorkspace := setupWorkspace(t, "workspace-2", "uid-2", "test-namespace")
		fakeClient := newIndexedWorkspaceClientBuilder(t, scheme).WithObjects(otherWorkspace).Build()
		handler := &WebhookHandler{Client: fakeClient, Decoder: admission.NewDecoder(scheme)}

		oldWorkspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
		newWorkspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
		// Unrelated change: the discoverable endpoint set is identical to oldWorkspace's.
		newWorkspace.Spec.Started = !oldWorkspace.Spec.Started

		req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Update,
			Object:    newRawExtension(t, newWorkspace),
			OldObject: newRawExtension(t, oldWorkspace),
		}}

		resp := handler.ValidateDevfile(context.TODO(), req)
		assert.True(t, resp.Allowed, "Update unrelated to endpoints must be allowed even though a real conflict exists, proving the check was skipped")
	})

	t.Run("Denies an update that introduces a new conflicting discoverable endpoint", func(t *testing.T) {
		otherWorkspace := setupWorkspace(t, "workspace-2", "uid-2", "test-namespace")
		fakeClient := newIndexedWorkspaceClientBuilder(t, scheme).WithObjects(otherWorkspace).Build()
		handler := &WebhookHandler{Client: fakeClient, Decoder: admission.NewDecoder(scheme)}

		oldWorkspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
		// Old workspace has no discoverable endpoints yet.
		oldWorkspace.Spec.Template.Components[0].Container.Endpoints[0].Attributes = nil

		// New workspace makes "test-endpoint" discoverable, which now conflicts with otherWorkspace.
		newWorkspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")

		req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Update,
			Object:    newRawExtension(t, newWorkspace),
			OldObject: newRawExtension(t, oldWorkspace),
		}}

		resp := handler.ValidateDevfile(context.TODO(), req)
		assert.False(t, resp.Allowed, "Update introducing a new conflicting discoverable endpoint must be denied")
		assert.Contains(t, resp.Result.Message, "test-endpoint")
		assert.Contains(t, resp.Result.Message, "workspace-2")
	})

	t.Run("Denies adding a unique endpoint while retaining an existing conflicting endpoint", func(t *testing.T) {
		otherWorkspace := setupWorkspace(t, "workspace-2", "uid-2", "test-namespace")
		fakeClient := newIndexedWorkspaceClientBuilder(t, scheme).WithObjects(otherWorkspace).Build()
		handler := &WebhookHandler{Client: fakeClient, Decoder: admission.NewDecoder(scheme)}
		oldWorkspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
		newWorkspace := oldWorkspace.DeepCopy()
		endpoint := newWorkspace.Spec.Template.Components[0].Container.Endpoints[0]
		endpoint.Name = "unique-endpoint"
		endpoint.TargetPort = 8081
		newWorkspace.Spec.Template.Components[0].Container.Endpoints = append(
			newWorkspace.Spec.Template.Components[0].Container.Endpoints, endpoint)
		req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Update,
			Object:    newRawExtension(t, newWorkspace),
			OldObject: newRawExtension(t, oldWorkspace),
		}}

		resp := handler.ValidateDevfile(context.TODO(), req)
		assert.False(t, resp.Allowed, "Adding a name must validate the full incoming endpoint set")
		assert.Contains(t, resp.Result.Message, "test-endpoint")
		assert.Contains(t, resp.Result.Message, "workspace-2")
	})
}

func TestValidateDevfileEndpointExposure(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, dwv2.AddToScheme(scheme))

	for _, tc := range []struct {
		name          string
		incoming      dwv2.EndpointExposure
		existing      dwv2.EndpointExposure
		allowed       bool
		withoutClient bool
	}{
		{name: "Incoming none allows a matching eligible endpoint", incoming: dwv2.NoneEndpointExposure, existing: dwv2.PublicEndpointExposure, allowed: true},
		{name: "Incoming none skips List", incoming: dwv2.NoneEndpointExposure, existing: dwv2.PublicEndpointExposure, allowed: true, withoutClient: true},
		{name: "Existing none does not block admission", incoming: dwv2.PublicEndpointExposure, existing: dwv2.NoneEndpointExposure, allowed: true},
		{name: "Internal endpoints conflict", incoming: dwv2.InternalEndpointExposure, existing: dwv2.InternalEndpointExposure},
		{name: "Public endpoints conflict", incoming: dwv2.PublicEndpointExposure, existing: dwv2.PublicEndpointExposure},
		{name: "Omitted exposure defaults to public and conflicts"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
			workspace.Spec.Template.Components[0].Container.Endpoints[0].Exposure = tc.incoming
			otherWorkspace := setupWorkspace(t, "workspace-2", "uid-2", "test-namespace")
			otherWorkspace.Spec.Template.Components[0].Container.Endpoints[0].Exposure = tc.existing
			handler := &WebhookHandler{Decoder: admission.NewDecoder(scheme)}
			if !tc.withoutClient {
				handler.Client = newIndexedWorkspaceClientBuilder(t, scheme).WithObjects(otherWorkspace).Build()
			}
			req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
				Operation: admissionv1.Create,
				Object:    newRawExtension(t, workspace),
			}}

			assert.NotPanics(t, func() {
				resp := handler.ValidateDevfile(context.TODO(), req)
				assert.Equal(t, tc.allowed, resp.Allowed, "Unexpected admission result: %v", resp.Result)
				if !tc.allowed {
					assert.Contains(t, resp.Result.Message, "test-endpoint")
					assert.Contains(t, resp.Result.Message, "workspace-2")
				}
			})
		})
	}
}

func TestValidateDevfileEndpointExposureChanges(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, dwv2.AddToScheme(scheme))

	for _, tc := range []struct {
		name   string
		before dwv2.EndpointExposure
		after  dwv2.EndpointExposure
		check  bool
	}{
		{"none to internal", dwv2.NoneEndpointExposure, dwv2.InternalEndpointExposure, true},
		{"none to public", dwv2.NoneEndpointExposure, dwv2.PublicEndpointExposure, true},
		{"none to omitted", dwv2.NoneEndpointExposure, "", true},
		{"internal to none", dwv2.InternalEndpointExposure, dwv2.NoneEndpointExposure, false},
		{"public to none", dwv2.PublicEndpointExposure, dwv2.NoneEndpointExposure, false},
		{"omitted to none", "", dwv2.NoneEndpointExposure, false},
		{"internal to public", dwv2.InternalEndpointExposure, dwv2.PublicEndpointExposure, false},
		{"public to internal", dwv2.PublicEndpointExposure, dwv2.InternalEndpointExposure, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oldWorkspace := setupWorkspace(t, "workspace-1", "uid-1", "test-namespace")
			oldWorkspace.Spec.Template.Components[0].Container.Endpoints[0].Exposure = tc.before
			newWorkspace := oldWorkspace.DeepCopy()
			newWorkspace.Spec.Template.Components[0].Container.Endpoints[0].Exposure = tc.after
			handler := &WebhookHandler{Decoder: admission.NewDecoder(scheme)}
			if tc.check {
				otherWorkspace := setupWorkspace(t, "workspace-2", "uid-2", "test-namespace")
				handler.Client = newIndexedWorkspaceClientBuilder(t, scheme).WithObjects(otherWorkspace).Build()
			}
			req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
				Operation: admissionv1.Update,
				Object:    newRawExtension(t, newWorkspace),
				OldObject: newRawExtension(t, oldWorkspace),
			}}

			assert.Equal(t, tc.check, handler.shouldCheckEndpointConflicts(req, discoverableEndpointNames(newWorkspace)))
			assert.NotPanics(t, func() {
				resp := handler.ValidateDevfile(context.TODO(), req)
				assert.Equal(t, !tc.check, resp.Allowed, "Unexpected admission result: %v", resp.Result)
				if tc.check {
					assert.Contains(t, resp.Result.Message, "test-endpoint")
					assert.Contains(t, resp.Result.Message, "workspace-2")
				}
			})
		})
	}
}
