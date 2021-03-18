//
// Copyright (c) 2019-2021 Red Hat, Inc.
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

	devworkspace "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	maputils "github.com/devfile/devworkspace-operator/internal/map"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"

	oauthv1 "github.com/openshift/api/oauth/v1"
	routeV1 "github.com/openshift/api/route/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type OpenShiftOAuthSolver struct {
	client.Client
}

var _ RoutingSolver = (*OpenShiftOAuthSolver)(nil)

type proxyEndpoint struct {
	machineName            string
	upstreamEndpoint       devworkspace.Endpoint
	publicEndpoint         devworkspace.Endpoint
	publicEndpointHttpPort int64
}

func (s *OpenShiftOAuthSolver) FinalizerRequired(*controllerv1alpha1.DevWorkspaceRouting) bool {
	return true
}

func (s *OpenShiftOAuthSolver) Finalize(routing *controllerv1alpha1.DevWorkspaceRouting) error {
	// Run finalization logic for devWorkspaceRoutingFinalizer. If the
	// finalization logic fails, don't remove the finalizer so
	// that we can retry during the next reconciliation.
	if err := deleteOAuthClients(s, routing); err != nil {
		return err
	}

	return nil
}

func (s *OpenShiftOAuthSolver) GetSpecObjects(routing *controllerv1alpha1.DevWorkspaceRouting, workspaceMeta WorkspaceMetadata) (RoutingObjects, error) {
	spec := routing.Spec
	proxy, noProxy := getProxiedEndpoints(spec)
	defaultRoutes := getRoutesForSpec(noProxy, workspaceMeta)

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
	discoverableServices := GetDiscoverableServicesForEndpoints(proxyPorts, workspaceMeta)
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
				constants.WorkspaceIDLabel: workspaceMeta.WorkspaceId,
			},
		},
		GrantMethod:  oauthv1.GrantHandlerPrompt,
		Secret:       "1234567890",
		RedirectURIs: publicURls,
	}

	restrictedAccess, setRestrictedAccess := routing.Annotations[constants.WorkspaceRestrictedAccessAnnotation]
	if setRestrictedAccess {
		oauthClient.Annotations = maputils.Append(oauthClient.Annotations, constants.WorkspaceRestrictedAccessAnnotation, restrictedAccess)
	}

	oauthClientInSync, err := syncOAuthClient(s, routing, oauthClient)
	if !oauthClientInSync {
		return RoutingObjects{}, &RoutingNotReady{}
	}
	if err != nil {
		return RoutingObjects{}, err
	}

	return RoutingObjects{
		Services:     services,
		Routes:       append(routes, defaultRoutes...),
		PodAdditions: podAdditions,
	}, nil
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
			route := getRouteForEndpoint(endpoint, workspaceMeta)
			route.Spec.TLS = &routeV1.TLSConfig{
				Termination:                   routeV1.TLSTerminationReencrypt,
				InsecureEdgeTerminationPolicy: routeV1.InsecureEdgeTerminationPolicyRedirect,
			}
			// Reverting single host feature since OpenShift OAuth uses absolute references
			route.Spec.Host = common.EndpointHostname(workspaceMeta.WorkspaceId, endpoint.Name, endpoint.TargetPort, workspaceMeta.RoutingSuffix)
			route.Spec.Path = "/"

			//override the original endpointName
			route.Annotations = maputils.Append(route.Annotations, constants.WorkspaceEndpointNameAnnotation, upstreamEndpoint.Name)
			routes = append(routes, route)
		}
	}
	podAdditions = getProxyPodAdditions(portMappings, workspaceMeta)
	return routes, podAdditions
}

func getProxiedEndpoints(spec controllerv1alpha1.DevWorkspaceRoutingSpec) (proxy, noProxy map[string]controllerv1alpha1.EndpointList) {
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
					Secure:     endpoint.Secure,
					Exposure:   endpoint.Exposure,
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
	endpointIsPublic := endpoint.Exposure == "" || endpoint.Exposure == devworkspace.PublicEndpointExposure
	return endpointIsPublic &&
		endpoint.Secure &&
		// Terminal is temporarily excluded from secure servers
		// because Theia is not aware how to authenticate against OpenShift OAuth
		endpoint.Attributes.Get(string(controllerv1alpha1.TypeEndpointAttribute), nil) != "terminal"
}
