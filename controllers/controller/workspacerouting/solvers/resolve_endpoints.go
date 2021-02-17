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
	"net/url"
	"strings"

	devworkspace "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
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
			endpointAttributes := map[string]string{}
			err := endpoint.Attributes.Into(&endpointAttributes)
			if err != nil {
				return nil, false, err
			}
			if endpoint.Exposure != devworkspace.PublicEndpointExposure {
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
				Attributes: endpointAttributes,
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
			return getURLForEndpoint(endpoint, route.Spec.Host, route.Spec.Path, route.Spec.TLS != nil), nil
		}
	}
	for _, ingress := range routingObj.Ingresses {
		if ingress.Annotations[config.WorkspaceEndpointNameAnnotation] == endpoint.Name {
			if len(ingress.Spec.Rules) == 1 {
				return getURLForEndpoint(endpoint, ingress.Spec.Rules[0].Host, "", false), nil // no TLS supported for ingresses yet
			} else {
				return "", fmt.Errorf("ingress %s contains multiple rules", ingress.Name)
			}
		}
	}
	return "", fmt.Errorf("could not find ingress/route for endpoint '%s'", endpoint.Name)
}

func getURLForEndpoint(endpoint devworkspace.Endpoint, host, basePath string, secure bool) string {
	protocol := endpoint.Protocol
	if secure && endpoint.Secure {
		protocol = devworkspace.EndpointProtocol(getSecureProtocol(string(protocol)))
	}
	var p string
	if endpoint.Path != "" {
		// the only one slash should be between these path segments.
		// Path.join does not suite here since it eats trailing slash which may be critical for the application
		p = fmt.Sprintf("%s/%s", strings.TrimRight(basePath, "/"), strings.TrimLeft(p, endpoint.Path))
	} else {
		p = basePath
	}
	u := url.URL{
		Scheme: string(protocol),
		Host:   host,
		Path:   p,
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
