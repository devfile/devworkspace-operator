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

package solvers

import (
	"testing"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestGetDiscoverableServicesForEndpoints(t *testing.T) {
	testLog := zap.New(zap.UseDevMode(true))
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = controllerv1alpha1.AddToScheme(scheme)

	discoverableEndpoint := controllerv1alpha1.Endpoint{
		Name:       "test-endpoint",
		TargetPort: 8080,
		Exposure:   controllerv1alpha1.PublicEndpointExposure,
		Attributes: controllerv1alpha1.Attributes{}.
			PutBoolean(string(controllerv1alpha1.DiscoverableAttribute), true),
	}
	endpoints := map[string]controllerv1alpha1.EndpointList{
		"machine1": {discoverableEndpoint},
	}

	meta := DevWorkspaceMetadata{
		DevWorkspaceId:   "current-workspace-id",
		DevWorkspaceName: "current-workspace",
		Namespace:        "test-namespace",
	}

	tests := []struct {
		name          string
		existing      []runtime.Object
		expectErr     bool
		expectErrType error
		expectMsg     string
	}{
		{
			name:      "No existing service",
			existing:  []runtime.Object{},
			expectErr: false,
		},
		{
			name: "Existing service with different owner in same namespace",
			existing: []runtime.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-endpoint",
						Namespace: "test-namespace",
						Labels: map[string]string{
							constants.DevWorkspaceIDLabel:   "other-workspace-id",
							constants.DevWorkspaceNameLabel: "other-workspace",
						},
					},
				},
			},
			expectErr:     true,
			expectErrType: &ServiceConflictError{},
			expectMsg:     "discoverable endpoint 'test-endpoint' is already in use by workspace 'other-workspace'",
		},
		{
			name: "Existing service with same owner (reconciliation)",
			existing: []runtime.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-endpoint",
						Namespace: "test-namespace",
						Labels: map[string]string{
							constants.DevWorkspaceIDLabel:   "current-workspace-id",
							constants.DevWorkspaceNameLabel: "current-workspace",
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Service with same name in different namespace (should not conflict)",
			existing: []runtime.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-endpoint",
						Namespace: "other-namespace",
						Labels: map[string]string{
							constants.DevWorkspaceIDLabel:   "other-workspace-id",
							constants.DevWorkspaceNameLabel: "other-workspace",
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Service without workspace ID label",
			existing: []runtime.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-endpoint",
						Namespace: "test-namespace",
						Labels:    map[string]string{},
					},
				},
			},
			expectErr:     true,
			expectErrType: &ServiceConflictError{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.existing...).Build()
			_, err := GetDiscoverableServicesForEndpoints(endpoints, meta, fakeClient, testLog)

			if tt.expectErr {
				assert.Error(t, err, "Expected an error but got none")
				if tt.expectErrType != nil {
					assert.IsType(t, tt.expectErrType, err, "Error is of unexpected type")
				}
				if tt.expectMsg != "" {
					assert.Contains(t, err.Error(), tt.expectMsg)
				}
			} else {
				assert.NoError(t, err, "Got unexpected error")
			}
		})
	}
}

func TestGetDiscoverableServicesForEndpoints_MultipleEndpoints(t *testing.T) {
	testLog := zap.New(zap.UseDevMode(true))
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = controllerv1alpha1.AddToScheme(scheme)

	endpoints := map[string]controllerv1alpha1.EndpointList{
		"machine1": {
			{
				Name:       "postgresql",
				TargetPort: 5432,
				Exposure:   controllerv1alpha1.InternalEndpointExposure,
				Attributes: controllerv1alpha1.Attributes{}.
					PutBoolean(string(controllerv1alpha1.DiscoverableAttribute), true),
			},
			{
				Name:       "redis",
				TargetPort: 6379,
				Exposure:   controllerv1alpha1.InternalEndpointExposure,
				Attributes: controllerv1alpha1.Attributes{}.
					PutBoolean(string(controllerv1alpha1.DiscoverableAttribute), true),
			},
			{
				Name:       "http",
				TargetPort: 8080,
				Exposure:   controllerv1alpha1.PublicEndpointExposure,
				// Not discoverable
			},
		},
	}

	meta := DevWorkspaceMetadata{
		DevWorkspaceId:   "current-workspace-id",
		DevWorkspaceName: "current-workspace",
		Namespace:        "test-namespace",
		PodSelector: map[string]string{
			constants.DevWorkspaceIDLabel: "current-workspace",
		},
	}

	t.Run("Multiple discoverable endpoints without conflicts", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		services, err := GetDiscoverableServicesForEndpoints(endpoints, meta, fakeClient, testLog)
		assert.NoError(t, err)
		assert.Len(t, services, 2, "Should create 2 discoverable services (postgresql and redis, not http)")

		serviceNames := make(map[string]bool)
		for _, svc := range services {
			serviceNames[svc.Name] = true
		}
		assert.True(t, serviceNames["postgresql"], "Should have postgresql service")
		assert.True(t, serviceNames["redis"], "Should have redis service")
		assert.False(t, serviceNames["http"], "Should not have http service (not discoverable)")
	})

	t.Run("Conflict on one of multiple endpoints", func(t *testing.T) {
		existingService := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "postgresql",
				Namespace: "test-namespace",
				Labels: map[string]string{
					constants.DevWorkspaceIDLabel:   "other-workspace-id",
					constants.DevWorkspaceNameLabel: "other-workspace",
				},
			},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(existingService).Build()
		_, err := GetDiscoverableServicesForEndpoints(endpoints, meta, fakeClient, testLog)
		assert.Error(t, err, "Should error when one endpoint conflicts")
		assert.IsType(t, &ServiceConflictError{}, err)
		assert.Contains(t, err.Error(), "postgresql")
	})
}
