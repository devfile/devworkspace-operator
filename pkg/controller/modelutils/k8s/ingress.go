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
	"fmt"
	"strconv"
	"strings"

	"github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/config"
)

// IngressHostname generates a hostname based on service name and namespace
func IngressHostname(serviceName, namespace string, port int64) string {
	ingressName := IngressName(serviceName, port)
	hostname := fmt.Sprintf("%s-%s", ingressName, namespace)
	if len(hostname) > 63 {
		hostname = strings.TrimSuffix(hostname[:63], "-")
	}
	return fmt.Sprintf("%s.%s", hostname, config.ControllerCfg.GetIngressGlobalDomain())
}

// IngressName generates a names for ingresses
func IngressName(serviceName string, port int64) string {
	portString := strconv.FormatInt(port, 10)
	return serviceName[0:10] + "-" + portString
}
