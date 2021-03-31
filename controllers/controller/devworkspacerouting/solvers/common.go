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
	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"

	routeV1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type DevWorkspaceMetadata struct {
	DevWorkspaceId string
	Namespace      string
	PodSelector    map[string]string
	RoutingSuffix  string
}

// GetDiscoverableServicesForEndpoints converts the endpoint list into a set of services, each corresponding to a single discoverable
// endpoint from the list. Endpoints with the NoneEndpointExposure are ignored.
func GetDiscoverableServicesForEndpoints(endpoints map[string]controllerv1alpha1.EndpointList, meta DevWorkspaceMetadata) []corev1.Service {
	var services []corev1.Service
	for _, machineEndpoints := range endpoints {
		for _, endpoint := range machineEndpoints {
			if endpoint.Exposure == dw.NoneEndpointExposure {
				continue
			}

			if endpoint.Attributes.GetBoolean(string(controllerv1alpha1.DiscoverableAttribute), nil) {
				// Create service with name matching endpoint
				// TODO: This could cause a reconcile conflict if multiple workspaces define the same discoverable endpoint
				// Also endpoint names may not be valid as service names
				servicePort := corev1.ServicePort{
					Name:       common.EndpointName(endpoint.Name),
					Protocol:   corev1.ProtocolTCP,
					Port:       int32(endpoint.TargetPort),
					TargetPort: intstr.FromInt(endpoint.TargetPort),
				}
				services = append(services, corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      common.EndpointName(endpoint.Name),
						Namespace: meta.Namespace,
						Labels: map[string]string{
							constants.DevWorkspaceIDLabel: meta.DevWorkspaceId,
						},
						Annotations: map[string]string{
							constants.DevWorkspaceDiscoverableServiceAnnotation: "true",
						},
					},
					Spec: corev1.ServiceSpec{
						Ports:    []corev1.ServicePort{servicePort},
						Selector: meta.PodSelector,
						Type:     corev1.ServiceTypeClusterIP,
					},
				})
			}
		}
	}
	return services
}

// GetServiceForEndpoints returns a single service that exposes all endpoints of given exposure types, possibly also including the discoverable types.
// `nil` is returned if the service would expose no ports satisfying the provided criteria.
func GetServiceForEndpoints(endpoints map[string]controllerv1alpha1.EndpointList, meta DevWorkspaceMetadata, includeDiscoverable bool, exposureType ...dw.EndpointExposure) *corev1.Service {
	// "set" of ports that are still left for exposure
	ports := map[int]bool{}
	for _, es := range endpoints {
		for _, endpoint := range es {
			ports[endpoint.TargetPort] = true
		}
	}

	// "set" of exposure types that are allowed
	validExposures := map[dw.EndpointExposure]bool{}
	for _, exp := range exposureType {
		validExposures[exp] = true
	}

	var exposedPorts []corev1.ServicePort

	for _, es := range endpoints {
		for _, endpoint := range es {
			if !validExposures[endpoint.Exposure] {
				continue
			}

			if !includeDiscoverable && endpoint.Attributes.GetBoolean(string(controllerv1alpha1.DiscoverableAttribute), nil) {
				continue
			}

			if ports[endpoint.TargetPort] {
				// make sure we don't mention the same port twice
				ports[endpoint.TargetPort] = false
				exposedPorts = append(exposedPorts, corev1.ServicePort{
					Name:       common.EndpointName(endpoint.Name),
					Protocol:   corev1.ProtocolTCP,
					Port:       int32(endpoint.TargetPort),
					TargetPort: intstr.FromInt(endpoint.TargetPort),
				})
			}
		}
	}

	if len(exposedPorts) == 0 {
		return nil
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ServiceName(meta.DevWorkspaceId),
			Namespace: meta.Namespace,
			Labels: map[string]string{
				constants.DevWorkspaceIDLabel: meta.DevWorkspaceId,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: meta.PodSelector,
			Type:     corev1.ServiceTypeClusterIP,
			Ports:    exposedPorts,
		},
	}
}

func getServicesForEndpoints(endpoints map[string]controllerv1alpha1.EndpointList, meta DevWorkspaceMetadata) []corev1.Service {
	if len(endpoints) == 0 {
		return nil
	}

	service := GetServiceForEndpoints(endpoints, meta, true, dw.PublicEndpointExposure, dw.InternalEndpointExposure)
	if service == nil {
		return nil
	}

	return []corev1.Service{
		*service,
	}
}

func getRoutesForSpec(endpoints map[string]controllerv1alpha1.EndpointList, meta DevWorkspaceMetadata) []routeV1.Route {
	var routes []routeV1.Route
	for _, machineEndpoints := range endpoints {
		for _, endpoint := range machineEndpoints {
			if endpoint.Exposure != dw.PublicEndpointExposure {
				continue
			}
			routes = append(routes, getRouteForEndpoint(endpoint, meta))
		}
	}
	return routes
}

func getIngressesForSpec(endpoints map[string]controllerv1alpha1.EndpointList, meta DevWorkspaceMetadata) []v1beta1.Ingress {
	var ingresses []v1beta1.Ingress
	for _, machineEndpoints := range endpoints {
		for _, endpoint := range machineEndpoints {
			if endpoint.Exposure != dw.PublicEndpointExposure {
				continue
			}
			ingresses = append(ingresses, getIngressForEndpoint(endpoint, meta))
		}
	}
	return ingresses
}

func getRouteForEndpoint(endpoint dw.Endpoint, meta DevWorkspaceMetadata) routeV1.Route {
	targetEndpoint := intstr.FromInt(endpoint.TargetPort)
	endpointName := common.EndpointName(endpoint.Name)
	return routeV1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.RouteName(meta.DevWorkspaceId, endpointName),
			Namespace: meta.Namespace,
			Labels: map[string]string{
				constants.DevWorkspaceIDLabel: meta.DevWorkspaceId,
			},
			Annotations: routeAnnotations(endpointName),
		},
		Spec: routeV1.RouteSpec{
			Host: common.WorkspaceHostname(meta.DevWorkspaceId, meta.RoutingSuffix),
			Path: common.EndpointPath(endpointName),
			TLS: &routeV1.TLSConfig{
				InsecureEdgeTerminationPolicy: routeV1.InsecureEdgeTerminationPolicyRedirect,
				Termination:                   routeV1.TLSTerminationEdge,
			},
			To: routeV1.RouteTargetReference{
				Kind: "Service",
				Name: common.ServiceName(meta.DevWorkspaceId),
			},
			Port: &routeV1.RoutePort{
				TargetPort: targetEndpoint,
			},
		},
	}
}

func getIngressForEndpoint(endpoint dw.Endpoint, meta DevWorkspaceMetadata) v1beta1.Ingress {
	targetEndpoint := intstr.FromInt(endpoint.TargetPort)
	endpointName := common.EndpointName(endpoint.Name)
	hostname := common.EndpointHostname(meta.DevWorkspaceId, endpointName, endpoint.TargetPort, meta.RoutingSuffix)
	ingressPathType := v1beta1.PathTypeImplementationSpecific
	return v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.RouteName(meta.DevWorkspaceId, endpointName),
			Namespace: meta.Namespace,
			Labels: map[string]string{
				constants.DevWorkspaceIDLabel: meta.DevWorkspaceId,
			},
			Annotations: nginxIngressAnnotations(endpoint.Name),
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: hostname,
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								{
									Backend: v1beta1.IngressBackend{
										ServiceName: common.ServiceName(meta.DevWorkspaceId),
										ServicePort: targetEndpoint,
									},
									PathType: &ingressPathType,
									Path:     "/",
								},
							},
						},
					},
				},
			},
		},
	}
}
