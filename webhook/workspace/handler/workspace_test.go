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
	"testing"

	dwv2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/httpfactory"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	authnv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func buildUpdateRequest(t *testing.T, oldWksp, newWksp *dwv2.DevWorkspace) admission.Request {
	t.Helper()

	oldRaw, err := json.Marshal(oldWksp)
	require.NoError(t, err)

	newRaw, err := json.Marshal(newWksp)
	require.NoError(t, err)

	gvk := dwv2.SchemeGroupVersion.WithKind("DevWorkspace")

	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       "test-uid",
			Namespace: oldWksp.Namespace,
			Operation: admissionv1.Update,
			Kind: metav1.GroupVersionKind{
				Group:   gvk.Group,
				Version: gvk.Version,
				Kind:    gvk.Kind,
			},
			Resource: metav1.GroupVersionResource{
				Group:    gvk.Group,
				Version:  gvk.Version,
				Resource: "devworkspaces",
			},
			UserInfo: authnv1.UserInfo{
				UID:      "test-user-uid",
				Username: "test-user",
			},
			OldObject: runtime.RawExtension{Raw: oldRaw},
			Object:    runtime.RawExtension{Raw: newRaw},
		},
	}
}

func newTestWebhookHandler(t *testing.T) *WebhookHandler {
	t.Helper()

	s := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(s))
	require.NoError(t, dwv2.AddToScheme(s))
	require.NoError(t, controllerv1alpha1.AddToScheme(s))

	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	config.SetGlobalConfigForTesting(&controllerv1alpha1.OperatorConfiguration{})
	httpfactory.SetupHttpClientsForTesting(nil)

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		Build()
	return &WebhookHandler{
		Client:  fakeClient,
		Decoder: admission.NewDecoder(s),
	}
}

func TestMutateWorkspaceV1alpha2OnUpdate_StopInvalidWorkspaceIsAllowed(t *testing.T) {
	handler := newTestWebhookHandler(t)

	oldWksp := getDevWorkspaceWithBrokenKubernetesComponent(true)
	newWksp := getDevWorkspaceWithBrokenKubernetesComponent(false)

	req := buildUpdateRequest(t, oldWksp, newWksp)
	resp := handler.MutateWorkspaceV1alpha2OnUpdate(context.Background(), req)

	assert.True(t, resp.Allowed)
	assert.NotEmpty(t, resp.Warnings, "expected a warning when stopping a workspace with permission errors")
}

func TestMutateWorkspaceV1alpha2OnUpdate_UpdateStartedInvalidWorkspaceIsDenied(t *testing.T) {
	handler := newTestWebhookHandler(t)

	oldWksp := getDevWorkspaceWithBrokenKubernetesComponent(true)
	newWksp := getDevWorkspaceWithBrokenKubernetesComponent(true)
	newWksp.Labels["updated"] = "true"

	req := buildUpdateRequest(t, oldWksp, newWksp)
	resp := handler.MutateWorkspaceV1alpha2OnUpdate(context.Background(), req)

	assert.False(t, resp.Allowed)
}

func TestMutateWorkspaceV1alpha2OnUpdate_UpdateAlreadyStoppedWorkspaceRunsValidation(t *testing.T) {
	handler := newTestWebhookHandler(t)

	oldWksp := getDevWorkspaceWithBrokenKubernetesComponent(false)
	newWksp := getDevWorkspaceWithBrokenKubernetesComponent(false)
	newWksp.Labels["updated"] = "true"

	req := buildUpdateRequest(t, oldWksp, newWksp)
	resp := handler.MutateWorkspaceV1alpha2OnUpdate(context.Background(), req)

	assert.False(t, resp.Allowed)
}

func TestMutateWorkspaceV1alpha2OnUpdate_StartingWorkspaceRunsValidation(t *testing.T) {
	handler := newTestWebhookHandler(t)

	oldWksp := getDevWorkspaceWithBrokenKubernetesComponent(false)
	newWksp := getDevWorkspaceWithBrokenKubernetesComponent(true)

	req := buildUpdateRequest(t, oldWksp, newWksp)
	resp := handler.MutateWorkspaceV1alpha2OnUpdate(context.Background(), req)

	assert.False(t, resp.Allowed, "starting a workspace with an invalid spec should be denied")
}

func getDevWorkspaceWithBrokenKubernetesComponent(started bool) *dwv2.DevWorkspace {
	return &dwv2.DevWorkspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workspace",
			Namespace: "test-namespace",
			Labels: map[string]string{
				constants.DevWorkspaceCreatorLabel: "test-user-uid",
			},
		},
		Spec: dwv2.DevWorkspaceSpec{
			Started: started,
			Template: dwv2.DevWorkspaceTemplateSpec{
				DevWorkspaceTemplateSpecContent: dwv2.DevWorkspaceTemplateSpecContent{
					Components: []dwv2.Component{
						{
							Name: "bad-k8s",
							ComponentUnion: dwv2.ComponentUnion{
								Kubernetes: &dwv2.KubernetesComponent{
									K8sLikeComponent: dwv2.K8sLikeComponent{
										K8sLikeComponentLocation: dwv2.K8sLikeComponentLocation{
											Inlined: "", // intentionally empty — triggers validation error
										},
										DeployByDefault: ptr.To(true),
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
