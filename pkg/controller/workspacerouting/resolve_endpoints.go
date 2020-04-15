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
			url, err := getURLforEndpoint(endpoint, ingresses, routes)
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

func getURLforEndpoint(
	endpoint workspacev1alpha1.Endpoint,
	ingresses []v1beta1.Ingress,
	routes []routeV1.Route) (string, error) {
	for _, route := range routes {
		if route.Annotations[config.WorkspaceEndpointNameAnnotation] == endpoint.Name {
			protocol := endpoint.Attributes[workspacev1alpha1.PROTOCOL_ENDPOINT_ATTRIBUTE]
			if endpoint.Attributes[workspacev1alpha1.SECURE_ENDPOINT_ATTRIBUTE] == "true" &&
				route.Spec.TLS != nil {
				protocol = getSecureProtocol(protocol)
			}
			url := fmt.Sprintf("%s://%s", protocol, route.Spec.Host)
			return url, nil
		}
	}
	for _, ingress := range ingresses {
		if ingress.Annotations[config.WorkspaceEndpointNameAnnotation] == endpoint.Name {
			if len(ingress.Spec.Rules) == 1 {
				protocol := endpoint.Attributes[workspacev1alpha1.PROTOCOL_ENDPOINT_ATTRIBUTE]
				url := fmt.Sprintf("%s://%s", protocol, ingress.Spec.Rules[0].Host)
				return url, nil
			} else {
				return "", fmt.Errorf("ingress %s contains multiple rules", ingress.Name)
			}
		}
	}
	return "", fmt.Errorf("could not find ingress/route for endpoint '%s'", endpoint.Name)
}
