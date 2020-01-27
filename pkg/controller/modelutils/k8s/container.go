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
