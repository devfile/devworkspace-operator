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

package common

import (
	"fmt"
	"regexp"
	"strings"
)

var NonAlphaNumRegexp = regexp.MustCompile(`[^a-z0-9]+`)

func EndpointName(endpointName string) string {
	name := strings.ToLower(endpointName)
	name = NonAlphaNumRegexp.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	return name
}

func ServiceName(workspaceId string) string {
	return fmt.Sprintf("%s-%s", workspaceId, "service")
}

func ServiceAccountName(workspaceId string) string {
	return fmt.Sprintf("%s-%s", workspaceId, "sa")
}

func EndpointHostname(workspaceId, endpointName string, endpointPort int, routingSuffix string) string {
	hostname := fmt.Sprintf("%s-%s-%d", workspaceId, endpointName, endpointPort)
	if len(hostname) > 63 {
		hostname = strings.TrimSuffix(hostname[:63], "-")
	}
	return fmt.Sprintf("%s.%s", hostname, routingSuffix)
}

func RouteName(workspaceId, endpointName string) string {
	return fmt.Sprintf("%s-%s", workspaceId, endpointName)
}

func CheRestAPIsConfigmapName(workspaceId string) string {
	return fmt.Sprintf("%s-%s", workspaceId, "che-rest-apis")
}

func PluginBrokerConfigmapName(workspaceId string) string {
	return fmt.Sprintf("%s-plugin-broker", workspaceId)
}

func OAuthProxySecretName(workspaceId string) string {
	return fmt.Sprintf("%s-%s", workspaceId, "proxy-tls")
}

func DeploymentName(workspaceId string) string {
	return workspaceId
}

func ServingCertVolumeName(serviceName string) string {
	return fmt.Sprintf("workspace-serving-cert-%s", serviceName)
}
