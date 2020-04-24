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

package workspacerouting

import (
	"fmt"
	"strings"

	workspacev1alpha1 "github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	"github.com/che-incubator/che-workspace-operator/pkg/config"
	routeV1 "github.com/openshift/api/route/v1"
	"k8s.io/api/extensions/v1beta1"
)

func getExposedEndpoints(
	endpoints map[string][]workspacev1alpha1.Endpoint,
	ingresses []v1beta1.Ingress,
	routes []routeV1.Route) (exposedEndpoints map[string][]workspacev1alpha1.ExposedEndpoint, ready bool, err error) {

	exposedEndpoints = map[string][]workspacev1alpha1.ExposedEndpoint{}
	ready = true

	for machineName, machineEndpoints := range endpoints {
		for _, endpoint := range machineEndpoints {
			if endpoint.Attributes[workspacev1alpha1.PUBLIC_ENDPOINT_ATTRIBUTE] != "true" {
				continue
			}
			url, err := resolveURLForEndpoint(endpoint, ingresses, routes)
			if err != nil {
				return nil, false, err
			}
			if url == "" {
				ready = false
			}
			exposedEndpoints[machineName] = append(exposedEndpoints[machineName], workspacev1alpha1.ExposedEndpoint{
				Name:       endpoint.Name,
				Url:        url,
				Attributes: endpoint.Attributes,
			})
		}
	}
	return exposedEndpoints, ready, nil
}

func resolveURLForEndpoint(
	endpoint workspacev1alpha1.Endpoint,
	ingresses []v1beta1.Ingress,
	routes []routeV1.Route) (string, error) {
	for _, route := range routes {
		if route.Annotations[config.WorkspaceEndpointNameAnnotation] == endpoint.Name {
			return getURLForEndpoint(endpoint, route.Spec.Host, route.Spec.TLS != nil), nil
		}
	}
	for _, ingress := range ingresses {
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

func getURLForEndpoint(endpoint workspacev1alpha1.Endpoint, host string, secure bool) string {
	protocol := endpoint.Attributes[workspacev1alpha1.PROTOCOL_ENDPOINT_ATTRIBUTE]
	if secure && endpoint.Attributes[workspacev1alpha1.SECURE_ENDPOINT_ATTRIBUTE] == "true" {
		protocol = getSecureProtocol(protocol)
	}
	path := endpoint.Attributes[workspacev1alpha1.PATH_ENDPOINT_ATTRIBUTE]
	if path != "" {
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
	}
	return fmt.Sprintf("%s://%s%s", protocol, host, path)
}
