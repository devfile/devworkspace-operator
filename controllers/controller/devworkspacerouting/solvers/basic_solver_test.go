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

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestServiceAnnotations(t *testing.T) {
	tests := []struct {
		name                 string
		sourceAnnotations    map[string]string
		serviceRoutingConfig v1alpha1.Service
		expectedAnnotations  map[string]string
	}{
		{
			name:              "No annotations provided should return empty",
			sourceAnnotations: nil,
			serviceRoutingConfig: v1alpha1.Service{
				Annotations: nil,
			},
			expectedAnnotations: map[string]string{},
		},
		{
			name: "Source annotations present should return source annotations",
			sourceAnnotations: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			serviceRoutingConfig: v1alpha1.Service{
				Annotations: nil,
			},
			expectedAnnotations: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name: "DevWorkspaceRouting Service routing config annotations merged with source annotations",
			sourceAnnotations: map[string]string{
				"key1": "value1",
			},
			serviceRoutingConfig: v1alpha1.Service{
				Annotations: map[string]string{
					"key3": "value3",
				},
			},
			expectedAnnotations: map[string]string{
				"key1": "value1",
				"key3": "value3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given + When
			result := mergeServiceAnnotations(tt.sourceAnnotations, tt.serviceRoutingConfig)
			// Then
			assert.Equal(t, tt.expectedAnnotations, result)
		})
	}
}

var devWorkspaceRouting = v1alpha1.DevWorkspaceRouting{
	Spec: v1alpha1.DevWorkspaceRoutingSpec{
		DevWorkspaceId: "workspaceb978dc9bd4ba428b",
		RoutingClass:   "basic",
		Endpoints: map[string]v1alpha1.EndpointList{
			"component1": []v1alpha1.Endpoint{
				{
					Name:       "endpoint1",
					TargetPort: 8080,
					Exposure:   "public",
					Protocol:   "http",
					Secure:     false,
					Path:       "/test",
					Attributes: map[string]apiext.JSON{},
					Annotations: map[string]string{
						"endpoint-annotation-key1": "endpoint-annotation-value1",
					},
				},
			},
		},
		PodSelector: map[string]string{
			"controller.devfile.io/devworkspace_id": "workspaceb978dc9bd4ba428b",
		},
		Service: map[string]v1alpha1.Service{
			"component1": {
				Annotations: map[string]string{
					"service-annotation-key": "service-annotation-value",
				},
			},
		},
	},
}

func TestGetSpecObjects_WhenValidDWRProvidedAndOpenShiftUnavailable_ThenGenerateRoutingObjectsServiceAndIngress(t *testing.T) {
	// Given
	basicSolver := &BasicSolver{}
	routingSuffixSupplier = func() string {
		return "test.routing"
	}
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	dwRouting := &devWorkspaceRouting
	workspaceMeta := DevWorkspaceMetadata{
		DevWorkspaceId: "workspaceb978dc9bd4ba428b",
		Namespace:      "test",
		PodSelector: map[string]string{
			"controller.devfile.io/devworkspace_id": "workspaceb978dc9bd4ba428b",
		},
	}

	// When
	routingObjects, err := basicSolver.GetSpecObjects(dwRouting, workspaceMeta)

	// Then
	assert.NotNil(t, routingObjects)
	assert.NoError(t, err)
	assert.Len(t, routingObjects.Services, 1)
	assert.Equal(t, corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "workspaceb978dc9bd4ba428b-service",
			Namespace:   "test",
			Labels:      map[string]string{"controller.devfile.io/devworkspace_id": "workspaceb978dc9bd4ba428b"},
			Annotations: map[string]string{"service-annotation-key": "service-annotation-value"},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "endpoint1",
					Protocol:   corev1.ProtocolTCP,
					Port:       8080,
					TargetPort: intstr.IntOrString{IntVal: 8080},
				},
			},
			Selector: map[string]string{"controller.devfile.io/devworkspace_id": "workspaceb978dc9bd4ba428b"},
		},
	}, routingObjects.Services[0])
	assert.Len(t, routingObjects.Ingresses, 1)
	assert.Equal(t, metav1.ObjectMeta{
		Name:      "workspaceb978dc9bd4ba428b-endpoint1",
		Namespace: "test",
		Labels:    map[string]string{"controller.devfile.io/devworkspace_id": "workspaceb978dc9bd4ba428b"},
		Annotations: map[string]string{
			"controller.devfile.io/endpoint_name":        "endpoint1",
			"endpoint-annotation-key1":                   "endpoint-annotation-value1",
			"nginx.ingress.kubernetes.io/rewrite-target": "/",
			"nginx.ingress.kubernetes.io/ssl-redirect":   "false",
		},
	}, routingObjects.Ingresses[0].ObjectMeta)
	assert.Len(t, routingObjects.Ingresses[0].Spec.Rules, 1)
	assert.Equal(t, "workspaceb978dc9bd4ba428b-endpoint1-8080.test.routing", routingObjects.Ingresses[0].Spec.Rules[0].Host)
	assert.Len(t, routingObjects.Ingresses[0].Spec.Rules[0].HTTP.Paths, 1)
	assert.Equal(t, networkingv1.IngressBackend{
		Service: &networkingv1.IngressServiceBackend{
			Name: "workspaceb978dc9bd4ba428b-service",
			Port: networkingv1.ServiceBackendPort{Number: int32(8080)},
		},
	}, routingObjects.Ingresses[0].Spec.Rules[0].HTTP.Paths[0].Backend)
	assert.Len(t, routingObjects.Routes, 0)
}

func TestGetSpecObjects_WhenValidDWRProvidedAndOpenShiftAvailable_ThenGenerateRoutingObjectsServiceAndRoute(t *testing.T) {
	// Given
	basicSolver := &BasicSolver{}
	routingSuffixSupplier = func() string {
		return "test.routing"
	}
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	dwRouting := &devWorkspaceRouting
	workspaceMeta := DevWorkspaceMetadata{
		DevWorkspaceId: "workspaceb978dc9bd4ba428b",
		Namespace:      "test",
		PodSelector: map[string]string{
			"controller.devfile.io/devworkspace_id": "workspaceb978dc9bd4ba428b",
		},
	}

	// When
	routingObjects, err := basicSolver.GetSpecObjects(dwRouting, workspaceMeta)

	// Then
	assert.NotNil(t, routingObjects)
	assert.NoError(t, err)
	assert.Len(t, routingObjects.Services, 1)
	assert.Equal(t, corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "workspaceb978dc9bd4ba428b-service",
			Namespace:   "test",
			Labels:      map[string]string{"controller.devfile.io/devworkspace_id": "workspaceb978dc9bd4ba428b"},
			Annotations: map[string]string{"service-annotation-key": "service-annotation-value"},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "endpoint1",
					Protocol:   corev1.ProtocolTCP,
					Port:       8080,
					TargetPort: intstr.IntOrString{IntVal: 8080},
				},
			},
			Selector: map[string]string{"controller.devfile.io/devworkspace_id": "workspaceb978dc9bd4ba428b"},
		},
	}, routingObjects.Services[0])
	assert.Len(t, routingObjects.Ingresses, 0)
	assert.Len(t, routingObjects.Routes, 1)
	assert.Equal(t, metav1.ObjectMeta{
		Name:      "workspaceb978dc9bd4ba428b-endpoint1",
		Namespace: "test",
		Labels: map[string]string{
			"controller.devfile.io/devworkspace_id": "workspaceb978dc9bd4ba428b",
		},
		Annotations: map[string]string{
			"controller.devfile.io/endpoint_name":        "endpoint1",
			"endpoint-annotation-key1":                   "endpoint-annotation-value1",
			"haproxy.router.openshift.io/rewrite-target": "/",
		},
	}, routingObjects.Routes[0].ObjectMeta)
	assert.Equal(t, "workspaceb978dc9bd4ba428b.test.routing", routingObjects.Routes[0].Spec.Host)
	assert.Equal(t, "/endpoint1/", routingObjects.Routes[0].Spec.Path)
	assert.Equal(t, "Service", routingObjects.Routes[0].Spec.To.Kind)
	assert.Equal(t, "workspaceb978dc9bd4ba428b-service", routingObjects.Routes[0].Spec.To.Name)
}
