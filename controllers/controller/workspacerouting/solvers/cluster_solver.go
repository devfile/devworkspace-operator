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

	"github.com/devfile/devworkspace-operator/pkg/common"

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha1"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config"
	corev1 "k8s.io/api/core/v1"
)

const (
	serviceServingCertAnnot = "service.beta.openshift.io/serving-cert-secret-name"
)

type ClusterSolver struct {
	TLS bool
}

var _ RoutingSolver = (*ClusterSolver)(nil)

func (s *ClusterSolver) GetSpecObjects(spec controllerv1alpha1.WorkspaceRoutingSpec, workspaceMeta WorkspaceMetadata) RoutingObjects {
	services := getServicesForEndpoints(spec.Endpoints, workspaceMeta)
	podAdditions := &controllerv1alpha1.PodAdditions{}
	if s.TLS {
		readOnlyMode := int32(420)
		for idx, service := range services {
			if services[idx].Annotations == nil {
				services[idx].Annotations = map[string]string{}
			}
			services[idx].Annotations[serviceServingCertAnnot] = service.Name
			podAdditions.Volumes = append(podAdditions.Volumes, corev1.Volume{
				Name: common.ServingCertVolumeName(service.Name),
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  service.Name,
						DefaultMode: &readOnlyMode,
					},
				},
			})
			podAdditions.VolumeMounts = append(podAdditions.VolumeMounts, corev1.VolumeMount{
				Name:      common.ServingCertVolumeName(service.Name),
				ReadOnly:  true,
				MountPath: "/var/serving-cert/",
			})
		}
	}

	return RoutingObjects{
		Services:     services,
		PodAdditions: podAdditions,
	}
}

func (s *ClusterSolver) GetExposedEndpoints(
	endpoints map[string]controllerv1alpha1.EndpointList,
	routingObj RoutingObjects) (exposedEndpoints map[string]controllerv1alpha1.ExposedEndpointList, ready bool, err error) {

	exposedEndpoints = map[string]controllerv1alpha1.ExposedEndpointList{}

	for machineName, machineEndpoints := range endpoints {
		for _, endpoint := range machineEndpoints {
			if endpoint.Attributes[string(controllerv1alpha1.PUBLIC_ENDPOINT_ATTRIBUTE)] != "true" {
				continue
			}
			url, err := resolveServiceHostnameForEndpoint(endpoint, routingObj.Services)
			if err != nil {
				return nil, false, err
			}
			exposedEndpoints[machineName] = append(exposedEndpoints[machineName], controllerv1alpha1.ExposedEndpoint{
				Name:       endpoint.Name,
				Url:        url,
				Attributes: endpoint.Attributes,
			})
		}
	}

	return exposedEndpoints, true, nil
}

func resolveServiceHostnameForEndpoint(endpoint devworkspace.Endpoint, services []corev1.Service) (string, error) {
	for _, service := range services {
		if service.Annotations[config.WorkspaceDiscoverableServiceAnnotation] == "true" {
			continue
		}
		for _, servicePort := range service.Spec.Ports {
			if servicePort.Port == int32(endpoint.TargetPort) {
				return getHostnameFromService(service, servicePort.Port), nil
			}
		}
	}
	return "", fmt.Errorf("could not find service for endpoint %s", endpoint.Name)
}

func getHostnameFromService(service corev1.Service, port int32) string {
	scheme := "http"
	if _, ok := service.Annotations[serviceServingCertAnnot]; ok {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s.%s.svc:%d", scheme, service.Name, service.Namespace, port)
}
