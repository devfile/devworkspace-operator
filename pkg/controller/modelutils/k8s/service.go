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
package utils

import (
	"strconv"
	"strings"

	"github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ContainerServiceName generates service names for workspaces
func ContainerServiceName(workspaceId, containerName string) string {
	return "server" + strings.ReplaceAll(workspaceId, "workspace", "") + "-" + containerName
}

// ServicePortName generates names for ports in workspace services
func ServicePortName(port int) string {
	return "server-" + strconv.FormatInt(int64(port), 10)
}

// BuildServicePorts converts exposed ports into k8s ServicePort objects
func BuildServicePorts(exposedPorts []int, protocol corev1.Protocol) []corev1.ServicePort {
	var servicePorts []corev1.ServicePort
	for _, port := range exposedPorts {
		servicePorts = append(servicePorts, corev1.ServicePort{
			Name:       ServicePortName(port),
			Protocol:   protocol,
			Port:       int32(port),
			TargetPort: intstr.FromInt(port),
		})
	}
	return servicePorts
}

// EndpointPortsToInts converts model endpoints to ints
func EndpointPortsToInts(endpoints []v1alpha1.Endpoint) []int {
	ports := []int{}
	for _, endpoint := range endpoints {
		ports = append(ports, int(endpoint.Port))
	}
	return ports
}
