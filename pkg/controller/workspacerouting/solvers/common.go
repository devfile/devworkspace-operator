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
	"github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	"github.com/che-incubator/che-workspace-operator/pkg/common"
	"github.com/che-incubator/che-workspace-operator/pkg/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type WorkspaceMetadata struct {
	WorkspaceId         string
	Namespace           string
	PodSelector         map[string]string
	IngressGlobalDomain string
}

func getDiscoverableServicesForEndpoints(endpoints map[string][]v1alpha1.Endpoint, workspaceMeta WorkspaceMetadata) []corev1.Service {
	var services []corev1.Service
	for _, machineEndpoints := range endpoints {
		for _, endpoint := range machineEndpoints {
			if endpoint.Attributes[v1alpha1.DISCOVERABLE_ATTRIBUTE] == "true" {
				// Create service with name matching endpoint
				// TODO: This could cause a reconcile conflict if multiple workspaces define the same discoverable endpoint
				// Also endpoint names may not be valid as service names
				servicePort := corev1.ServicePort{
					Name:       common.EndpointName(endpoint.Name),
					Protocol:   corev1.ProtocolTCP,
					Port:       int32(endpoint.Port),
					TargetPort: intstr.FromInt(int(endpoint.Port)),
				}
				services = append(services, corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      common.EndpointName(endpoint.Name),
						Namespace: workspaceMeta.Namespace,
						Labels: map[string]string{
							config.WorkspaceIDLabel: workspaceMeta.WorkspaceId,
						},
					},
					Spec: corev1.ServiceSpec{
						Ports:    []corev1.ServicePort{servicePort},
						Selector: workspaceMeta.PodSelector,
						Type:     corev1.ServiceTypeClusterIP,
					},
				})
			}
		}
	}
	return services
}

func getServicesForEndpoints(endpoints map[string][]v1alpha1.Endpoint, workspaceMeta WorkspaceMetadata) []corev1.Service {
	var services []corev1.Service
	var servicePorts []corev1.ServicePort
	for _, machineEndpoints := range endpoints {
		for _, endpoint := range machineEndpoints {
			servicePort := corev1.ServicePort{
				Name:       common.EndpointName(endpoint.Name),
				Protocol:   corev1.ProtocolTCP,
				Port:       int32(endpoint.Port),
				TargetPort: intstr.FromInt(int(endpoint.Port)),
			}
			servicePorts = append(servicePorts, servicePort)
		}
	}

	services = append(services, corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ServiceName(workspaceMeta.WorkspaceId),
			Namespace: workspaceMeta.Namespace,
			Labels: map[string]string{
				config.WorkspaceIDLabel: workspaceMeta.WorkspaceId,
			},
		},
		Spec: corev1.ServiceSpec{
			Ports:    servicePorts,
			Selector: workspaceMeta.PodSelector,
			Type:     corev1.ServiceTypeClusterIP,
		},
	})

	return services
}

func getIngressesForSpec(endpoints map[string][]v1alpha1.Endpoint, workspaceMeta WorkspaceMetadata) ([]v1beta1.Ingress, map[string][]v1alpha1.ExposedEndpoint) {
	var ingresses []v1beta1.Ingress
	exposedEndpoints := map[string][]v1alpha1.ExposedEndpoint{}

	for machineName, machineEndpoints := range endpoints {
		for _, endpoint := range machineEndpoints {
			if endpoint.Attributes[v1alpha1.PUBLIC_ENDPOINT_ATTRIBUTE] != "true" {
				continue
			}
			// Note: there is an additional limitation on target endpoint here: must be a DNS name fewer than 15 chars long
			// In general, endpoint.Name _cannot_ be used here
			var targetEndpoint intstr.IntOrString
			targetEndpoint = intstr.FromInt(int(endpoint.Port))

			endpointName := common.EndpointName(endpoint.Name)
			ingressHostname := common.EndpointHostname(workspaceMeta.WorkspaceId, endpointName, endpoint.Port, workspaceMeta.IngressGlobalDomain)
			ingressURL := fmt.Sprintf("http://%s", ingressHostname)
			ingresses = append(ingresses, v1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.RouteName(workspaceMeta.WorkspaceId, endpointName),
					Namespace: workspaceMeta.Namespace,
					Labels: map[string]string{
						config.WorkspaceIDLabel: workspaceMeta.WorkspaceId,
					},
					Annotations: ingressAnnotations,
				},
				Spec: v1beta1.IngressSpec{
					Rules: []v1beta1.IngressRule{
						{
							Host: ingressHostname,
							IngressRuleValue: v1beta1.IngressRuleValue{
								HTTP: &v1beta1.HTTPIngressRuleValue{
									Paths: []v1beta1.HTTPIngressPath{
										{
											Backend: v1beta1.IngressBackend{
												ServiceName: common.ServiceName(workspaceMeta.WorkspaceId),
												ServicePort: targetEndpoint,
											},
										},
									},
								},
							},
						},
					},
				},
			})
			exposedEndpoints[machineName] = append(exposedEndpoints[machineName], v1alpha1.ExposedEndpoint{
				Name:       endpoint.Name,
				Url:        ingressURL,
				Attributes: endpoint.Attributes,
			})
		}
	}
	return ingresses, exposedEndpoints
}
