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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"strconv"
)

func BuildContainerPorts(exposedPorts []int, protocol corev1.Protocol) []corev1.ContainerPort {
	containerPorts := []corev1.ContainerPort{}
	for _, exposedPort := range exposedPorts {
		containerPorts = append(containerPorts, corev1.ContainerPort{
			ContainerPort: int32(exposedPort),
			Protocol:      protocol,
		})
	}
	if len(containerPorts) == 0 {
		return nil
	}
	return containerPorts
}

func ServicePortName(port int) string {
	return "srv-" + strconv.FormatInt(int64(port), 10)
}

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
