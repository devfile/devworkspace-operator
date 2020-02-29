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

type ComponentDescription struct {
	// WorkspacePodAdditions contains the workspace's contributions to the main deployment (e.g. containers, volumes, etc.)
	WorkspaceAdditions *ComponentWorkspaceAdditions `json:"-"`
	// ExternalObjects represents the additional (non-deployment) objects that are required for the workspace (e.g. Services)
	ExternalObjects ComponentExternalObjects `json:"-"`
	// ComponentStatus holds metadata about the component that is serialized into the workspace's status
	Status ComponentStatus `json:",inline"`
}

type ComponentWorkspaceAdditions struct {
	Annotations    map[string]string
	Labels         map[string]string
	Containers     []corev1.Container
	InitContainers []corev1.Container
	Volumes        []corev1.Volume
	PullSecrets    []corev1.LocalObjectReference
}

type ComponentExternalObjects []runtime.Object

type ComponentStatus struct {
	// Containers is a map of container names to ContainerDescriptions. Field is serialized into workspace status "additionalFields"
	// and consumed by che-rest-apis
	Containers map[string]ContainerDescription `json:"containers,omitempty"`
	// ContributedRuntimeCommands represent the devfile commands available in the current workspace. They are serialized into the
	// workspace status "additionalFields" and consumed by che-rest-apis.
	ContributedRuntimeCommands []CheWorkspaceCommand `json:"contributedRuntimeCommands,omitempty"`
	// Endpoints stores the workspace endpoints defined by the component
	Endpoints []workspacev1alpha1.Endpoint `json:"-"`
}

// ContainerDescription stores metadata about workspace containers. This is used to provide information
// to Theia via the che-rest-apis container.
type ContainerDescription struct {
	// Attributes stores the Che-specific metadata about a component, e.g. a plugin's ID, memoryLimit from devfile, etc.
	Attributes map[string]string `json:"attributes,omitempty"`
	// Ports stores the list of ports exposed by this container.
	Ports []int `json:"ports,omitempty"`
}
