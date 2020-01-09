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
	"github.com/eclipse/che-plugin-broker/model"
)

func ExposedPortsToInts(exposedPorts []model.ExposedPort) []int {
	ports := []int{}
	for _, exposedPort := range exposedPorts {
		ports = append(ports, exposedPort.ExposedPort)
	}
	return ports
}
