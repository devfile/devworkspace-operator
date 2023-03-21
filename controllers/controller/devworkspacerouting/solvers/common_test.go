package solvers

import (
	"testing"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestGetRouteForEndpointAnnotations(t *testing.T) {
	tests := []struct {
		name string

		routingSuffix string
		endpoint      v1alpha1.Endpoint
		meta          DevWorkspaceMetadata
		annotations   map[string]string

		expectedAnnotationsKeys []string
	}{
		{
			name: "nil",

			annotations: nil,

			expectedAnnotationsKeys: []string{
				"controller.devfile.io/endpoint_name",
				"haproxy.router.openshift.io/rewrite-target",
			},
		},
		{
			name: "empty",

			annotations: map[string]string{},

			expectedAnnotationsKeys: []string{
				"controller.devfile.io/endpoint_name",
				"haproxy.router.openshift.io/rewrite-target",
			},
		},
		{
			name: "defined",

			annotations: map[string]string{
				"example.com/extra": "val",
			},

			expectedAnnotationsKeys: []string{
				"controller.devfile.io/endpoint_name",
				"example.com/extra"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := getRouteForEndpoint("routingSuffix", v1alpha1.Endpoint{Name: "Endpoint"}, DevWorkspaceMetadata{DevWorkspaceId: "WorkspaceTest"}, tt.annotations)
			for _, expected := range tt.expectedAnnotationsKeys {
				_, ok := route.Annotations[expected]
				assert.True(t, ok, "Key %s does not exist", expected)
				assert.Equal(t, len(tt.expectedAnnotationsKeys), len(route.Annotations))
			}
		})
	}
}

func TestGetIngressForEndpointAnnotations(t *testing.T) {
	tests := []struct {
		name string

		routingSuffix string
		endpoint      v1alpha1.Endpoint
		meta          DevWorkspaceMetadata
		annotations   map[string]string

		expectedAnnotationsKeys []string
	}{
		{
			name: "nil",

			annotations: nil,

			expectedAnnotationsKeys: []string{
				"controller.devfile.io/endpoint_name",
				"kubernetes.io/ingress.class",
				"nginx.ingress.kubernetes.io/rewrite-target",
				"nginx.ingress.kubernetes.io/ssl-redirect",
			},
		},
		{
			name: "empty",

			annotations: map[string]string{},

			expectedAnnotationsKeys: []string{
				"controller.devfile.io/endpoint_name",
				"kubernetes.io/ingress.class",
				"nginx.ingress.kubernetes.io/rewrite-target",
				"nginx.ingress.kubernetes.io/ssl-redirect",
			},
		},
		{
			name: "defined",

			annotations: map[string]string{
				"kubernetes.io/ingress.class": "traefik",
			},

			expectedAnnotationsKeys: []string{
				"controller.devfile.io/endpoint_name",
				"kubernetes.io/ingress.class",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingress := getIngressForEndpoint("routingSuffix", v1alpha1.Endpoint{Name: "Endpoint"}, DevWorkspaceMetadata{DevWorkspaceId: "WorkspaceTest"}, tt.annotations)
			for _, expected := range tt.expectedAnnotationsKeys {
				_, ok := ingress.Annotations[expected]
				assert.True(t, ok, "Key %s does not exist", expected)
				assert.Equal(t, len(tt.expectedAnnotationsKeys), len(ingress.Annotations))
			}
		})
	}
}
