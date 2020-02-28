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

import (
	"fmt"
	"github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
)

func ProxyRouteName(serviceName string, endpoint v1alpha1.Endpoint) string {
	return fmt.Sprintf("%s-%s", serviceName, endpoint.Name)
}

func ProxyContainerName(endpoint v1alpha1.Endpoint) string {
	return "oauth-proxy-" + endpoint.Name
}

func EndpointNeedsProxy(endpoint v1alpha1.Endpoint) bool {
	publicAttr, exists := endpoint.Attributes[v1alpha1.PUBLIC_ENDPOINT_ATTRIBUTE]
	endpointIsPublic := !exists || (publicAttr == "true")
	return endpointIsPublic &&
		endpoint.Attributes[v1alpha1.SECURE_ENDPOINT_ATTRIBUTE] == "true" &&
		endpoint.Attributes[v1alpha1.TYPE_ENDPOINT_ATTRIBUTE] != "terminal"
}
