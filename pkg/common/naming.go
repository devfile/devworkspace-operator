//
// Copyright (c) 2019-2023 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package common

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

var NonAlphaNumRegexp = regexp.MustCompile(`[^a-z0-9]+`)

func DevWorkspaceRoutingName(workspaceId string) string {
	return fmt.Sprintf("routing-%s", workspaceId)
}

func EndpointName(endpointName string) string {
	name := strings.ToLower(endpointName)
	name = NonAlphaNumRegexp.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	return name
}

func PortName(endpoint dw.Endpoint) string {
	if len(endpoint.Name) <= 15 {
		return endpoint.Name
	}
	return fmt.Sprintf("%d-%s", endpoint.TargetPort, endpoint.Protocol)
}

func ServiceName(workspaceId string) string {
	return fmt.Sprintf("%s-%s", workspaceId, "service")
}

func ServiceAccountName(workspace *DevWorkspaceWithConfig) string {
	if workspace.Config.Workspace.ServiceAccount.ServiceAccountName != "" {
		return workspace.Config.Workspace.ServiceAccount.ServiceAccountName
	}
	return fmt.Sprintf("%s-%s", workspace.Status.DevWorkspaceId, "sa")
}

func ServiceAccountLabels(workspace *DevWorkspaceWithConfig) map[string]string {
	if workspace.Config.Workspace.ServiceAccount.ServiceAccountName != "" {
		// One SA used for multiple workspaces; do not add specific DevWorkspace ID. We still need the
		// devworkspace ID label in order to cache the ServiceAccount.
		return map[string]string{
			constants.DevWorkspaceIDLabel: "",
		}
	}
	return map[string]string{
		constants.DevWorkspaceIDLabel:   workspace.Status.DevWorkspaceId,
		constants.DevWorkspaceNameLabel: workspace.Name,
	}
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

func PerWorkspacePVCName(workspaceId string) string {
	return fmt.Sprintf("storage-%s", workspaceId)
}

func MetadataConfigMapName(workspaceId string) string {
	return fmt.Sprintf("%s-metadata", workspaceId)
}

// We can't add prefixes to automount volume names, as adding any characters
// can potentially push the name over the 63 character limit (if the original
// object has a long name)
func AutoMountConfigMapVolumeName(volumeName string) string {
	return volumeName
}

func AutoMountSecretVolumeName(volumeName string) string {
	return volumeName
}

func AutoMountPVCVolumeName(pvcName string) string {
	return pvcName
}

func AutoMountProjectedVolumeName(mountPath string) string {
	// To avoid issues around sanitizing mountPath to generate a unique name (length, allowed chars)
	// just use the sha256 hash of mountPath
	hash := sha256.Sum256([]byte(mountPath))
	return fmt.Sprintf("projected-%x", hash[:10])
}

func ServiceAccountTokenProjectionName(mountPath string) string {
	hash := sha256.Sum256([]byte(mountPath))
	return fmt.Sprintf("sa-token-projected-%x", hash[:10])
}

func WorkspaceRoleName() string {
	return "devworkspace-default-role"
}

func WorkspaceRolebindingName() string {
	return "devworkspace-default-rolebinding"
}

func WorkspaceSCCRoleName(sccName string) string {
	return fmt.Sprintf("devworkspace-use-%s", sccName)
}

func WorkspaceSCCRolebindingName(sccName string) string {
	return fmt.Sprintf("devworkspace-use-%s", sccName)
}

// OldWorkspaceRoleName returns the name used for the workspace serviceaccount role
//
// Deprecated: use WorkspaceRoleName() instead.
// TODO: remove for DevWorkspace Operator v0.19
func OldWorkspaceRoleName() string {
	return "workspace"
}

// OldWorkspaceRolebindingName returns the name used for the workspace serviceaccount rolebinding
//
// Deprecated: use WorkspaceRoleBindingName() instead.
// TODO: remove for DevWorkspace Operator v0.19
func OldWorkspaceRolebindingName() string {
	return constants.ServiceAccount + "dw"
}
