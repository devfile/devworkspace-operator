//
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

package solvers

import (
	"fmt"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// GatewayAPISolver exposes endpoints using Kubernetes Gateway API HTTPRoutes
// HTTPRoutes are attached to a Gateway specified in the operator configuration
type GatewayAPISolver struct{}

var _ RoutingSolver = (*GatewayAPISolver)(nil)

func (s *GatewayAPISolver) FinalizerRequired(*controllerv1alpha1.DevWorkspaceRouting) bool {
	return false
}

func (s *GatewayAPISolver) Finalize(*controllerv1alpha1.DevWorkspaceRouting) error {
	return nil
}

func (s *GatewayAPISolver) GetSpecObjects(routing *controllerv1alpha1.DevWorkspaceRouting, workspaceMeta DevWorkspaceMetadata) (RoutingObjects, error) {
	routingObjects := RoutingObjects{}

	// Validate Gateway reference is configured
	globalConfig := config.GetGlobalConfig()
	if globalConfig.Routing == nil || globalConfig.Routing.GatewayRef == nil {
		return routingObjects, &RoutingInvalid{"gateway-api routing requires .config.routing.gatewayRef to be set in operator config"}
	}

	// Validate cluster host suffix
	routingSuffix := globalConfig.Routing.ClusterHostSuffix
	if routingSuffix == "" {
		return routingObjects, &RoutingInvalid{"gateway-api routing requires .config.routing.clusterHostSuffix to be set in operator config"}
	}

	spec := routing.Spec

	// Create Services (same as basic solver - HTTPRoutes route to Services)
	services := getServicesForEndpoints(spec.Endpoints, workspaceMeta)
	services = append(services, GetDiscoverableServicesForEndpoints(spec.Endpoints, workspaceMeta)...)
	routingObjects.Services = services

	// Create HTTPRoutes for Gateway API
	httpRoutes := s.getHTTPRoutesForSpec(globalConfig.Routing.GatewayRef, routing.Namespace, routingSuffix, spec.Endpoints, workspaceMeta)
	routingObjects.HTTPRoutes = httpRoutes

	return routingObjects, nil
}

func (s *GatewayAPISolver) getHTTPRoutesForSpec(
	gatewayRef *controllerv1alpha1.GatewayReference,
	routingNamespace string,
	clusterHostSuffix string,
	endpoints map[string]controllerv1alpha1.EndpointList,
	workspaceMeta DevWorkspaceMetadata,
) []gwapiv1.HTTPRoute {
	var httpRoutes []gwapiv1.HTTPRoute

	// Determine Gateway namespace - use same namespace as routing if not specified
	gatewayNamespace := routingNamespace
	if gatewayRef.Namespace != nil {
		gatewayNamespace = *gatewayRef.Namespace
	}

	for _, machineEndpoints := range endpoints {
		for _, endpoint := range machineEndpoints {
			if endpoint.Exposure != controllerv1alpha1.PublicEndpointExposure {
				continue
			}

			// Generate hostname using common naming function
			endpointName := common.EndpointName(endpoint.Name)
			hostname := common.EndpointHostname(clusterHostSuffix, workspaceMeta.DevWorkspaceId, endpointName, endpoint.TargetPort)

			// Create HTTP -> HTTPS redirect route (port 80)
			httpRedirectRoute := s.createHTTPRedirectRoute(
				gatewayRef,
				gatewayNamespace,
				hostname,
				endpoint,
				endpointName,
				workspaceMeta,
			)
			httpRoutes = append(httpRoutes, httpRedirectRoute)

			// Create HTTPS backend route (port 443)
			httpsRoute := s.createHTTPSBackendRoute(
				gatewayRef,
				gatewayNamespace,
				hostname,
				endpoint,
				endpointName,
				workspaceMeta,
			)
			httpRoutes = append(httpRoutes, httpsRoute)
		}
	}

	return httpRoutes
}

func (s *GatewayAPISolver) createHTTPRedirectRoute(
	gatewayRef *controllerv1alpha1.GatewayReference,
	gatewayNamespace string,
	hostname string,
	endpoint controllerv1alpha1.Endpoint,
	endpointName string,
	workspaceMeta DevWorkspaceMetadata,
) gwapiv1.HTTPRoute {
	routeName := fmt.Sprintf("%s-http-redirect", common.RouteName(workspaceMeta.DevWorkspaceId, endpointName))

	httpsScheme := "https"
	statusCode := 308

	pathPrefix := gwapiv1.PathMatchPathPrefix
	pathValue := "/"

	group := gwapiv1.Group(gwapiv1.GroupVersion.Group)
	kind := gwapiv1.Kind("Gateway")
	namespace := gwapiv1.Namespace(gatewayNamespace)
	port := gwapiv1.PortNumber(80)

	return gwapiv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeName,
			Namespace: workspaceMeta.Namespace,
			Labels: map[string]string{
				constants.DevWorkspaceIDLabel: workspaceMeta.DevWorkspaceId,
			},
			Annotations: map[string]string{
				constants.DevWorkspaceEndpointNameAnnotation: endpoint.Name,
			},
		},
		Spec: gwapiv1.HTTPRouteSpec{
			CommonRouteSpec: gwapiv1.CommonRouteSpec{
				ParentRefs: []gwapiv1.ParentReference{
					{
						Group:     &group,
						Kind:      &kind,
						Namespace: &namespace,
						Name:      gwapiv1.ObjectName(gatewayRef.Name),
						Port:      &port,
					},
				},
			},
			Hostnames: []gwapiv1.Hostname{gwapiv1.Hostname(hostname)},
			Rules: []gwapiv1.HTTPRouteRule{
				{
					Matches: []gwapiv1.HTTPRouteMatch{
						{
							Path: &gwapiv1.HTTPPathMatch{
								Type:  &pathPrefix,
								Value: &pathValue,
							},
						},
					},
					Filters: []gwapiv1.HTTPRouteFilter{
						{
							Type: gwapiv1.HTTPRouteFilterRequestRedirect,
							RequestRedirect: &gwapiv1.HTTPRequestRedirectFilter{
								Scheme:     &httpsScheme,
								StatusCode: &statusCode,
							},
						},
					},
				},
			},
		},
	}
}

func (s *GatewayAPISolver) createHTTPSBackendRoute(
	gatewayRef *controllerv1alpha1.GatewayReference,
	gatewayNamespace string,
	hostname string,
	endpoint controllerv1alpha1.Endpoint,
	endpointName string,
	workspaceMeta DevWorkspaceMetadata,
) gwapiv1.HTTPRoute {
	routeName := common.RouteName(workspaceMeta.DevWorkspaceId, endpointName)

	pathPrefix := gwapiv1.PathMatchPathPrefix
	pathValue := "/"

	// 10 hour timeout (matching ingress2gateway output and long-running workspace sessions)
	requestTimeout := gwapiv1.Duration("10h")

	group := gwapiv1.Group(gwapiv1.GroupVersion.Group)
	kind := gwapiv1.Kind("Gateway")
	namespace := gwapiv1.Namespace(gatewayNamespace)
	httpsPort := gwapiv1.PortNumber(443)
	servicePort := gwapiv1.PortNumber(endpoint.TargetPort)

	return gwapiv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeName,
			Namespace: workspaceMeta.Namespace,
			Labels: map[string]string{
				constants.DevWorkspaceIDLabel: workspaceMeta.DevWorkspaceId,
			},
			Annotations: map[string]string{
				constants.DevWorkspaceEndpointNameAnnotation: endpoint.Name,
			},
		},
		Spec: gwapiv1.HTTPRouteSpec{
			CommonRouteSpec: gwapiv1.CommonRouteSpec{
				ParentRefs: []gwapiv1.ParentReference{
					{
						Group:     &group,
						Kind:      &kind,
						Namespace: &namespace,
						Name:      gwapiv1.ObjectName(gatewayRef.Name),
						Port:      &httpsPort,
					},
				},
			},
			Hostnames: []gwapiv1.Hostname{gwapiv1.Hostname(hostname)},
			Rules: []gwapiv1.HTTPRouteRule{
				{
					Matches: []gwapiv1.HTTPRouteMatch{
						{
							Path: &gwapiv1.HTTPPathMatch{
								Type:  &pathPrefix,
								Value: &pathValue,
							},
						},
					},
					BackendRefs: []gwapiv1.HTTPBackendRef{
						{
							BackendRef: gwapiv1.BackendRef{
								BackendObjectReference: gwapiv1.BackendObjectReference{
									Name: gwapiv1.ObjectName(common.ServiceName(workspaceMeta.DevWorkspaceId)),
									Port: &servicePort,
								},
							},
						},
					},
					Timeouts: &gwapiv1.HTTPRouteTimeouts{
						Request: &requestTimeout,
					},
				},
			},
		},
	}
}

func (s *GatewayAPISolver) GetExposedEndpoints(
	endpoints map[string]controllerv1alpha1.EndpointList,
	routingObj RoutingObjects) (exposedEndpoints map[string]controllerv1alpha1.ExposedEndpointList, ready bool, err error) {

	// For Gateway API, we can construct URLs immediately from HTTPRoutes
	// Similar to how Ingresses work - the URL is determined by the hostname in the HTTPRoute spec
	return getExposedEndpointsFromHTTPRoutes(endpoints, routingObj)
}

// getExposedEndpointsFromHTTPRoutes extracts endpoint URLs from HTTPRoutes
func getExposedEndpointsFromHTTPRoutes(
	endpoints map[string]controllerv1alpha1.EndpointList,
	routingObj RoutingObjects) (exposedEndpoints map[string]controllerv1alpha1.ExposedEndpointList, ready bool, err error) {

	exposedEndpoints = map[string]controllerv1alpha1.ExposedEndpointList{}
	ready = true

	// Iterate through endpoints and match them to HTTPRoutes by annotation
	for componentName, endpointList := range endpoints {
		for _, endpoint := range endpointList {
			if endpoint.Exposure != controllerv1alpha1.PublicEndpointExposure {
				continue
			}

			// Find the HTTPRoute for this endpoint by matching the endpoint name annotation
			var url string
			for _, httpRoute := range routingObj.HTTPRoutes {
				// Only check HTTPS routes (not HTTP redirects) - they're the actual backend routes
				if httpRoute.Annotations[constants.DevWorkspaceEndpointNameAnnotation] == endpoint.Name {
					if len(httpRoute.Spec.Rules) > 0 && len(httpRoute.Spec.Rules[0].BackendRefs) > 0 {
						if len(httpRoute.Spec.Hostnames) > 0 {
							hostname := string(httpRoute.Spec.Hostnames[0])
							// Use HTTPS scheme
							url = fmt.Sprintf("https://%s%s", hostname, endpoint.Path)
							break
						}
					}
				}
			}

			if url == "" {
				ready = false
			}

			exposedEndpoints[componentName] = append(exposedEndpoints[componentName],
				controllerv1alpha1.ExposedEndpoint{
					Name:       endpoint.Name,
					Url:        url,
					Attributes: endpoint.Attributes,
				})
		}
	}

	return exposedEndpoints, ready, nil
}
