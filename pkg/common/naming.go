//
// Copyright (c) 2019-2021 Red Hat, Inc.
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

func EndpointHostname(routingSuffix, workspaceId, endpointName string, endpointPort int) string {
	hostname := fmt.Sprintf("%s-%s-%d", workspaceId, endpointName, endpointPort)
	if len(hostname) > 63 {
		hostname = strings.TrimSuffix(hostname[:63], "-")
	}
	return fmt.Sprintf("%s.%s", hostname, routingSuffix)
}

// WorkspaceHostname evaluates a single hostname for a workspace, and should be used for routing
// when endpoints are distinguished by path rules
func WorkspaceHostname(routingSuffix, workspaceId string) string {
	hostname := workspaceId
	if len(hostname) > 63 {
		hostname = strings.TrimSuffix(hostname[:63], "-")
	}
	return fmt.Sprintf("%s.%s", hostname, routingSuffix)
}

func EndpointPath(endpointName string) string {
	return "/" + endpointName + "/"
}

func RouteName(workspaceId, endpointName string) string {
	return fmt.Sprintf("%s-%s", workspaceId, endpointName)
}

func DeploymentName(workspaceId string) string {
	return workspaceId
}

func ServingCertVolumeName(serviceName string) string {
	return fmt.Sprintf("devworkspace-serving-cert-%s", serviceName)
}

func PVCCleanupJobName(workspaceId string) string {
	return fmt.Sprintf("cleanup-%s", workspaceId)
}

func MetadataConfigMapName(workspaceId string) string {
	return fmt.Sprintf("%s-metadata", workspaceId)
}

func AutoMountConfigMapVolumeName(volumeName string) string {
	return fmt.Sprintf("automount-configmap-%s", volumeName)
}

func AutoMountSecretVolumeName(volumeName string) string {
	return fmt.Sprintf("automount-secret-%s", volumeName)
}

func AutoMountPVCVolumeName(pvcName string) string {
	return fmt.Sprintf("automount-pvc-%s", pvcName)
}
