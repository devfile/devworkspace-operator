package solvers

import (
	"testing"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
)

func TestGetRouteForEndpointAnnotations(t *testing.T) {
	tests := []struct {
		name string

		annotations map[string]string

		expectedAnnotations map[string]string
	}{
		{
			name: "Gets default OpenShift route annotation when annotations aren't defined",

			annotations: nil,

			expectedAnnotations: map[string]string{
				"controller.devfile.io/endpoint_name":        "Endpoint",
				"haproxy.router.openshift.io/rewrite-target": "/",
			},
		},
		{
			name: "Gets default OpenShift route annotation when annotations are empty",

			annotations: map[string]string{},

			expectedAnnotations: map[string]string{
				"controller.devfile.io/endpoint_name":        "Endpoint",
				"haproxy.router.openshift.io/rewrite-target": "/",
			},
		},
		{
			name: "Gets default OpenShift route annotation when annotations are defined",

			annotations: map[string]string{
				"example.com/extra": "val",
			},

			expectedAnnotations: map[string]string{
				"controller.devfile.io/endpoint_name": "Endpoint",
				"example.com/extra":                   "val",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := getRouteForEndpoint("routingSuffix", v1alpha1.Endpoint{Name: "Endpoint"}, DevWorkspaceMetadata{DevWorkspaceId: "WorkspaceTest"}, tt.annotations)
			assert.Equal(t, tt.expectedAnnotations, route.Annotations, "Annotations should match: Diff: %s", cmp.Diff(tt.expectedAnnotations, route.Annotations))
		})
	}
}

func TestGetIngressForEndpointAnnotations(t *testing.T) {
	tests := []struct {
		name string

		annotations map[string]string

		expectedAnnotations map[string]string
	}{
		{
			name: "Gets default Kubernetes ingress annotation when annotations aren't defined",

			annotations: nil,

			expectedAnnotations: map[string]string{
				"controller.devfile.io/endpoint_name":        "Endpoint",
				"kubernetes.io/ingress.class":                "nginx",
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
				"nginx.ingress.kubernetes.io/ssl-redirect":   "false",
			},
		},
		{
			name: "Gets default Kubernetes ingress annotation when annotations are empty",

			annotations: map[string]string{},

			expectedAnnotations: map[string]string{
				"controller.devfile.io/endpoint_name":        "Endpoint",
				"kubernetes.io/ingress.class":                "nginx",
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
				"nginx.ingress.kubernetes.io/ssl-redirect":   "false",
			},
		},
		{
			name: "Gets default Kubernetes ingress annotation when annotations are defined",

			annotations: map[string]string{
				"kubernetes.io/ingress.class": "traefik",
			},

			expectedAnnotations: map[string]string{
				"controller.devfile.io/endpoint_name": "Endpoint",
				"kubernetes.io/ingress.class":         "traefik",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingress := getIngressForEndpoint("routingSuffix", v1alpha1.Endpoint{Name: "Endpoint"}, DevWorkspaceMetadata{DevWorkspaceId: "WorkspaceTest"}, tt.annotations)
			assert.Equal(t, tt.expectedAnnotations, ingress.Annotations, "Annotations should match: Diff: %s", cmp.Diff(tt.expectedAnnotations, ingress.Annotations))
		})
	}
}
