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

package model

import (
	workspacev1alpha1 "github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ContainerDescription stores metadata about workspace containers. This is used to provide information
// to Theia via the che-rest-apis container.
type ContainerDescription struct {
	// Attributes stores the Che-specific metadata about a component, e.g. a plugin's ID, memoryLimit from devfile, etc.
	Attributes map[string]string `json:"attributes,omitempty"`
	// Ports stores the list of ports exposed by this container.
	Ports []int `json:"ports,omitempty"`
}

// ComponentInstanceStatus represents a workspace components contributions to a workspace deployment, along with additional
// metadata that is required by che-rest-apis. Parts of this struct are serialized into the status of the workspace custom
// resource.
type ComponentInstanceStatus struct {
	// Containers is a map of container names to ContainerDescriptions. Field is serialized into workspace status "additionalFields"
	// and consumed by che-rest-apis
	Containers map[string]ContainerDescription `json:"containers,omitempty"`
	// WorkspacePodAdditions contains the workspace's contributions to the main deployment (e.g. containers, volumes, etc.)
	WorkspacePodAdditions *corev1.PodTemplateSpec `json:"-"`
	// ExternalObjects represents the additional (non-deployment) objects that are required for the workspace (e.g. Services)
	ExternalObjects []runtime.Object `json:"-"`
	// Endpoints stores the workspace endpoints defined by the component
	Endpoints []workspacev1alpha1.Endpoint `json:"-"`
	// ContributedRuntimeCommands represent the devfile commands available in the current workspace. They are serialized into the
	// workspace status "additionalFields" and consumed by che-rest-apis.
	ContributedRuntimeCommands []CheWorkspaceCommand `json:"contributedRuntimeCommands,omitempty"`
}

//type ComponentDescription struct {
//	WorkspaceAdditions ComponentWorkspaceAdditions `json:"-"`
//	ExternalObjects ComponentExternalObjects `json:"-"`
//	ComponentStatus ContainerDescription `json:"containers,omitempty"`
//}
//
//// ComponentWorkspaceAdditions contains the k8s elements that should be added to the main workspace pod
//type ComponentWorkspaceAdditions struct {
//	Attributes map[string]string
//	Labels     map[string]string
//
//}
//
//// ComponentExternalObjects contains the external k8s objects that the component contributes to the workspace
//type ComponentExternalObjects struct {
//}
