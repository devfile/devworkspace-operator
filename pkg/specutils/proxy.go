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

package specutils

func ProxyServiceName(serviceName string, port int64) string {
	return IngressName(serviceName, port) + "-oauth-proxy"
}

func ProxyServiceAccountName(routingName string) string {
	return routingName + "-oauth-proxy"
}

func ProxyDeploymentName(routingName string) string {
	return routingName + "-oauth-proxy"
}

