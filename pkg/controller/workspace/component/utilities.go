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

package component

import (
	"strings"

	workspaceApi "github.com/che-incubator/che-workspace-crd-operator/pkg/apis/workspace/v1alpha1"
	. "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/config"
	. "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/model"
)

func emptyIfNil(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

func machineServiceName(wkspProps WorkspaceProperties, machineName string) string {
	return "server"+strings.ReplaceAll(wkspProps.WorkspaceId, "workspace", "") + "-" + machineName
}

func endpointPortsToInts(endpoints []workspaceApi.Endpoint) []int {
	ports := []int {}
	for _, endpint := range endpoints {
		ports = append(ports, int(endpint.Port))
	}
	return ports
}

func ingressHostName(name string, wkspProperties WorkspaceProperties) string {
	return name + "-" + wkspProperties.Namespace + "." + ControllerCfg.GetIngressGlobalDomain()
}
