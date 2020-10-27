//
// Copyright (c) 2019-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package solvers

import (
	"fmt"

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha1"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/config"
	oauthv1 "github.com/openshift/api/oauth/v1"
	routeV1 "github.com/openshift/api/route/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type OpenShiftOAuthSolver struct{}

var _ RoutingSolver = (*OpenShiftOAuthSolver)(nil)

type proxyEndpoint struct {
	machineName            string
	upstreamEndpoint       devworkspace.Endpoint
	publicEndpoint         devworkspace.Endpoint
	publicEndpointHttpPort int64
}

func (s *OpenShiftOAuthSolver) GetSpecObjects(spec controllerv1alpha1.WorkspaceRoutingSpec, workspaceMeta WorkspaceMetadata) RoutingObjects {
	proxy, noProxy := getProxiedEndpoints(spec)
	defaultIngresses, defaultRoutes := getRoutingForSpec(noProxy, workspaceMeta)

	portMappings := getProxyEndpointMappings(proxy)
	var proxyPorts = map[string]controllerv1alpha1.EndpointList{}
	for _, proxyEndpoint := range portMappings {
		proxyPorts[proxyEndpoint.machineName] = append(proxyPorts[proxyEndpoint.machineName], proxyEndpoint.publicEndpoint)
	}
	for machineName, machineEndpoints := range noProxy {
		proxyPorts[machineName] = append(proxyPorts[machineName], machineEndpoints...)
	}
	// Use common service for all unproxied endpoints
	proxyServices := getServicesForEndpoints(proxyPorts, workspaceMeta)
	for idx := range proxyServices {
		proxyServices[idx].Annotations = map[string]string{
			"service.alpha.openshift.io/serving-cert-secret-name": common.OAuthProxySecretName(workspaceMeta.WorkspaceId),
		}
	}
	discoverableServices := getDiscoverableServicesForEndpoints(proxyPorts, workspaceMeta)
	services := append(proxyServices, discoverableServices...)

	routes, podAdditions := s.getProxyRoutes(proxy, workspaceMeta, portMappings)

	var publicURls []string
	for _, route := range routes {
		publicURls = append(publicURls, "https://"+route.Spec.Host+"/oauth/callback")
	}

	oauthClient := &oauthv1.OAuthClient{
		ObjectMeta: metav1.ObjectMeta{
			Name: workspaceMeta.WorkspaceId + "-oauth-client",
			Labels: map[string]string{
				config.WorkspaceIDLabel: workspaceMeta.WorkspaceId,
			},
		},
		GrantMethod:  oauthv1.GrantHandlerPrompt,
		Secret:       "1234567890",
		RedirectURIs: publicURls,
	}

	return RoutingObjects{
		Services:     services,
		Ingresses:    defaultIngresses,
		Routes:       append(routes, defaultRoutes...),
		PodAdditions: podAdditions,
		OAuthClient:  oauthClient,
	}
}

func (s *OpenShiftOAuthSolver) GetExposedEndpoints(
	endpoints map[string]controllerv1alpha1.EndpointList,
	routingObj RoutingObjects) (exposedEndpoints map[string]controllerv1alpha1.ExposedEndpointList, ready bool, err error) {
	return getExposedEndpoints(endpoints, routingObj)
}

func (s *OpenShiftOAuthSolver) getProxyRoutes(
	endpoints map[string]controllerv1alpha1.EndpointList,
	workspaceMeta WorkspaceMetadata,
	portMappings map[string]proxyEndpoint) ([]routeV1.Route, *controllerv1alpha1.PodAdditions) {

	var routes []routeV1.Route
	var podAdditions *controllerv1alpha1.PodAdditions
	for _, machineEndpoints := range endpoints {
		for _, upstreamEndpoint := range machineEndpoints {
			proxyEndpoint := portMappings[upstreamEndpoint.Name]
			endpoint := proxyEndpoint.publicEndpoint
			var tls *routeV1.TLSConfig = nil
			if endpoint.Attributes[string(controllerv1alpha1.SECURE_ENDPOINT_ATTRIBUTE)] == "true" {
				if endpoint.Attributes[string(controllerv1alpha1.TYPE_ENDPOINT_ATTRIBUTE)] == "terminal" {
					tls = &routeV1.TLSConfig{
						Termination:                   routeV1.TLSTerminationEdge,
						InsecureEdgeTerminationPolicy: routeV1.InsecureEdgeTerminationPolicyRedirect,
					}
				} else {
					tls = &routeV1.TLSConfig{
						Termination:                   routeV1.TLSTerminationReencrypt,
						InsecureEdgeTerminationPolicy: routeV1.InsecureEdgeTerminationPolicyRedirect,
					}
				}
			}
			route := getRouteForEndpoint(endpoint, workspaceMeta)
			route.Spec.TLS = tls
			if route.Annotations == nil {
				route.Annotations = map[string]string{}
			}
			route.Annotations[config.WorkspaceEndpointNameAnnotation] = upstreamEndpoint.Name
			routes = append(routes, route)
		}
	}
	podAdditions = getProxyPodAdditions(portMappings, workspaceMeta)
	return routes, podAdditions
}

func getProxiedEndpoints(spec controllerv1alpha1.WorkspaceRoutingSpec) (proxy, noProxy map[string]controllerv1alpha1.EndpointList) {
	proxy = map[string]controllerv1alpha1.EndpointList{}
	noProxy = map[string]controllerv1alpha1.EndpointList{}
	for machineName, machineEndpoints := range spec.Endpoints {
		for _, endpoint := range machineEndpoints {
			if endpointNeedsProxy(endpoint) {
				proxy[machineName] = append(proxy[machineName], endpoint)
			} else {
				noProxy[machineName] = append(noProxy[machineName], endpoint)
			}
		}
	}
	return
}

func getProxyEndpointMappings(
	endpoints map[string]controllerv1alpha1.EndpointList) map[string]proxyEndpoint {
	proxyHttpsPort := 4400
	proxyHttpPort := int64(4180)

	var proxyEndpoints = map[string]proxyEndpoint{}
	for machineName, machineEndpoints := range endpoints {
		for _, endpoint := range machineEndpoints {
			proxyEndpoints[endpoint.Name] = proxyEndpoint{
				machineName:      machineName,
				upstreamEndpoint: endpoint,
				publicEndpoint: devworkspace.Endpoint{
					Attributes: endpoint.Attributes,
					Name:       fmt.Sprintf("%s-proxy", endpoint.Name),
					TargetPort: proxyHttpsPort,
				},
				publicEndpointHttpPort: proxyHttpPort,
			}
			proxyHttpsPort++
			proxyHttpPort++
		}
	}

	return proxyEndpoints
}

func endpointNeedsProxy(endpoint devworkspace.Endpoint) bool {
	publicAttr, exists := endpoint.Attributes[string(controllerv1alpha1.PUBLIC_ENDPOINT_ATTRIBUTE)]
	endpointIsPublic := !exists || (publicAttr == "true")
	return endpointIsPublic &&
		endpoint.Attributes[string(controllerv1alpha1.SECURE_ENDPOINT_ATTRIBUTE)] == "true" &&
		endpoint.Attributes[string(controllerv1alpha1.TYPE_ENDPOINT_ATTRIBUTE)] != "terminal"
}
