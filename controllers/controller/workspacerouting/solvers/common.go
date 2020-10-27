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
	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha1"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/config"
	routeV1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type WorkspaceMetadata struct {
	WorkspaceId   string
	Namespace     string
	PodSelector   map[string]string
	RoutingSuffix string
}

func getDiscoverableServicesForEndpoints(endpoints map[string]controllerv1alpha1.EndpointList, meta WorkspaceMetadata) []corev1.Service {
	var services []corev1.Service
	for _, machineEndpoints := range endpoints {
		for _, endpoint := range machineEndpoints {
			if endpoint.Attributes[string(controllerv1alpha1.DISCOVERABLE_ATTRIBUTE)] == "true" {
				// Create service with name matching endpoint
				// TODO: This could cause a reconcile conflict if multiple workspaces define the same discoverable endpoint
				// Also endpoint names may not be valid as service names
				servicePort := corev1.ServicePort{
					Name:       common.EndpointName(endpoint.Name),
					Protocol:   corev1.ProtocolTCP,
					Port:       int32(endpoint.TargetPort),
					TargetPort: intstr.FromInt(int(endpoint.TargetPort)),
				}
				services = append(services, corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      common.EndpointName(endpoint.Name),
						Namespace: meta.Namespace,
						Labels: map[string]string{
							config.WorkspaceIDLabel: meta.WorkspaceId,
						},
						Annotations: map[string]string{
							config.WorkspaceDiscoverableServiceAnnotation: "true",
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

func getServicesForEndpoints(endpoints map[string]controllerv1alpha1.EndpointList, meta WorkspaceMetadata) []corev1.Service {
	var services []corev1.Service
	var servicePorts []corev1.ServicePort
	for _, machineEndpoints := range endpoints {
		for _, endpoint := range machineEndpoints {
			isExposed := func(port int) bool {
				for _, port := range servicePorts {
					if port.TargetPort.IntVal == int32(endpoint.TargetPort) {
						//port is already exposed
						return true
					}
				}
				return false
			}

			if !isExposed(endpoint.TargetPort) {
				servicePort := corev1.ServicePort{
					Name:       common.EndpointName(endpoint.Name),
					Protocol:   corev1.ProtocolTCP,
					Port:       int32(endpoint.TargetPort),
					TargetPort: intstr.FromInt(endpoint.TargetPort),
				}
				servicePorts = append(servicePorts, servicePort)
			}
		}
	}

	services = append(services, corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ServiceName(meta.WorkspaceId),
			Namespace: meta.Namespace,
			Labels: map[string]string{
				config.WorkspaceIDLabel: meta.WorkspaceId,
			},
		},
		Spec: corev1.ServiceSpec{
			Ports:    servicePorts,
			Selector: meta.PodSelector,
			Type:     corev1.ServiceTypeClusterIP,
		},
	})

	return services
}

func getRoutingForSpec(endpoints map[string]controllerv1alpha1.EndpointList, meta WorkspaceMetadata) ([]v1beta1.Ingress, []routeV1.Route) {
	var ingresses []v1beta1.Ingress
	var routes []routeV1.Route
	for _, machineEndpoints := range endpoints {
		for _, endpoint := range machineEndpoints {
			if endpoint.Attributes[string(controllerv1alpha1.PUBLIC_ENDPOINT_ATTRIBUTE)] != "true" {
				continue
			}
			if config.ControllerCfg.IsOpenShift() {
				routes = append(routes, getRouteForEndpoint(endpoint, meta))
			} else {
				ingresses = append(ingresses, getIngressForEndpoint(endpoint, meta))
			}
		}
	}
	return ingresses, routes
}

func getRouteForEndpoint(endpoint devworkspace.Endpoint, meta WorkspaceMetadata) routeV1.Route {
	targetEndpoint := intstr.FromInt(int(endpoint.TargetPort))
	endpointName := common.EndpointName(endpoint.Name)
	return routeV1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.RouteName(meta.WorkspaceId, endpointName),
			Namespace: meta.Namespace,
			Labels: map[string]string{
				config.WorkspaceIDLabel: meta.WorkspaceId,
			},
			Annotations: map[string]string{
				config.WorkspaceEndpointNameAnnotation: endpoint.Name,
			},
		},
		Spec: routeV1.RouteSpec{
			Host: common.EndpointHostname(meta.WorkspaceId, endpointName, endpoint.TargetPort, meta.RoutingSuffix),
			To: routeV1.RouteTargetReference{
				Kind: "Service",
				Name: common.ServiceName(meta.WorkspaceId),
			},
			Port: &routeV1.RoutePort{
				TargetPort: targetEndpoint,
			},
		},
	}
}

func getIngressForEndpoint(endpoint devworkspace.Endpoint, meta WorkspaceMetadata) v1beta1.Ingress {
	targetEndpoint := intstr.FromInt(int(endpoint.TargetPort))
	endpointName := common.EndpointName(endpoint.Name)
	hostname := common.EndpointHostname(meta.WorkspaceId, endpointName, endpoint.TargetPort, meta.RoutingSuffix)
	annotations := map[string]string{
		config.WorkspaceEndpointNameAnnotation: endpoint.Name,
	}
	for k, v := range ingressAnnotations {
		annotations[k] = v
	}
	ingressPathType := v1beta1.PathTypeImplementationSpecific
	return v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.RouteName(meta.WorkspaceId, endpointName),
			Namespace: meta.Namespace,
			Labels: map[string]string{
				config.WorkspaceIDLabel: meta.WorkspaceId,
			},
			Annotations: annotations,
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
										ServiceName: common.ServiceName(meta.WorkspaceId),
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
