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

package v1alpha1

import (
	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha1"
)

// Description of a devfile component's workspace additions
type ComponentDescription struct {
	// The name of the component
	Name string `json:"name"`
	// Additions to the workspace pod
	PodAdditions PodAdditions `json:"podAdditions"`
	// Additional metadata from devfile (e.g. attributes, commands)
	ComponentMetadata ComponentMetadata `json:"componentMetadata"`
}

type ComponentMetadata struct {
	// Containers is a map of container names to ContainerDescriptions. Field is serialized into workspace status "additionalFields"
	// and consumed by che-rest-apis
	Containers map[string]ContainerDescription `json:"containers,omitempty"`
	// ContributedRuntimeCommands represent the devfile commands available in the current workspace. They are serialized into the
	// workspace status "additionalFields" and consumed by che-rest-apis.
	ContributedRuntimeCommands []CheWorkspaceCommand `json:"contributedRuntimeCommands,omitempty"`
	// Endpoints stores the workspace endpoints defined by the component
	Endpoints []devworkspace.Endpoint `json:"endpoints,omitempty"`
}

// ContainerDescription stores metadata about workspace containers. This is used to provide information
// to Theia via the che-rest-apis container.
type ContainerDescription struct {
	// Attributes stores the Che-specific metadata about a component, e.g. a plugin's ID, memoryLimit from devfile, etc.
	Attributes map[string]string `json:"attributes,omitempty"`
	// Ports stores the list of ports exposed by this container.
	Ports []int `json:"ports,omitempty"`
}

// Command to add to workspace
type CheWorkspaceCommand struct {
	// Name of the command
	Name string `json:"name"`
	// Type of the command
	Type string `json:"type"`
	// String representing the commandline to be executed
	CommandLine string `json:"commandLine"`
	// Attributes for command
	Attributes map[string]string `json:"attributes,omitempty"`
}
