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
	"net/url"

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha1"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config"
)

func getExposedEndpoints(
	endpoints map[string]controllerv1alpha1.EndpointList,
	routingObj RoutingObjects) (exposedEndpoints map[string]controllerv1alpha1.ExposedEndpointList, ready bool, err error) {

	exposedEndpoints = map[string]controllerv1alpha1.ExposedEndpointList{}
	ready = true

	for machineName, machineEndpoints := range endpoints {
		for _, endpoint := range machineEndpoints {
			if endpoint.Attributes[string(controllerv1alpha1.PUBLIC_ENDPOINT_ATTRIBUTE)] != "true" {
				continue
			}
			endpointUrl, err := resolveURLForEndpoint(endpoint, routingObj)
			if err != nil {
				return nil, false, err
			}
			if endpointUrl == "" {
				ready = false
			}
			exposedEndpoints[machineName] = append(exposedEndpoints[machineName], controllerv1alpha1.ExposedEndpoint{
				Name:       endpoint.Name,
				Url:        endpointUrl,
				Attributes: endpoint.Attributes,
			})
		}
	}
	return exposedEndpoints, ready, nil
}

func resolveURLForEndpoint(
	endpoint devworkspace.Endpoint,
	routingObj RoutingObjects) (string, error) {
	for _, route := range routingObj.Routes {
		if route.Annotations[config.WorkspaceEndpointNameAnnotation] == endpoint.Name {
			return getURLForEndpoint(endpoint, route.Spec.Host, route.Spec.TLS != nil), nil
		}
	}
	for _, ingress := range routingObj.Ingresses {
		if ingress.Annotations[config.WorkspaceEndpointNameAnnotation] == endpoint.Name {
			if len(ingress.Spec.Rules) == 1 {
				return getURLForEndpoint(endpoint, ingress.Spec.Rules[0].Host, false), nil // no TLS supported for ingresses yet
			} else {
				return "", fmt.Errorf("ingress %s contains multiple rules", ingress.Name)
			}
		}
	}
	return "", fmt.Errorf("could not find ingress/route for endpoint '%s'", endpoint.Name)
}

func getURLForEndpoint(endpoint devworkspace.Endpoint, host string, secure bool) string {
	protocol := endpoint.Attributes[string(controllerv1alpha1.PROTOCOL_ENDPOINT_ATTRIBUTE)]
	if secure && endpoint.Attributes[string(controllerv1alpha1.SECURE_ENDPOINT_ATTRIBUTE)] == "true" {
		protocol = getSecureProtocol(protocol)
	}
	path := endpoint.Attributes[string(controllerv1alpha1.PATH_ENDPOINT_ATTRIBUTE)]
	u := url.URL{
		Scheme: protocol,
		Host:   host,
		Path:   path,
	}
	return u.String()
}

// getSecureProtocol takes a (potentially unsecure protocol e.g. http) and returns the secure version (e.g. https).
// If protocol isn't recognized, it is returned unmodified.
func getSecureProtocol(protocol string) string {
	switch protocol {
	case "ws":
		return "wss"
	case "http":
		return "https"
	default:
		return protocol
	}
}
