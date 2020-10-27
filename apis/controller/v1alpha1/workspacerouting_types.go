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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WorkspaceRoutingSpec defines the desired state of WorkspaceRouting
// +k8s:openapi-gen=true
type WorkspaceRoutingSpec struct {
	// WorkspaceId for the workspace being routed
	WorkspaceId string `json:"workspaceId"`
	// Class of the routing: this drives which Workspace Routing controller will manage this routing
	RoutingClass WorkspaceRoutingClass `json:"routingClass,omitempty"`
	// Routing suffix for cluster
	RoutingSuffix string `json:"routingSuffix"`
	// Machines to endpoints map
	Endpoints map[string]EndpointList `json:"endpoints"`
	// Selector that should be used by created services to point to the workspace Pod
	PodSelector map[string]string `json:"podSelector"`
}

type WorkspaceRoutingClass string

const (
	WorkspaceRoutingDefault        WorkspaceRoutingClass = "basic"
	WorkspaceRoutingOpenShiftOauth WorkspaceRoutingClass = "openshift-oauth"
	WorkspaceRoutingCluster        WorkspaceRoutingClass = "cluster"
	WorkspaceRoutingClusterTLS     WorkspaceRoutingClass = "cluster-tls"
	WorkspaceRoutingWebTerminal    WorkspaceRoutingClass = "web-terminal"
)

// WorkspaceRoutingStatus defines the observed state of WorkspaceRouting
// +k8s:openapi-gen=true
type WorkspaceRoutingStatus struct {
	// Additions to main workspace deployment
	PodAdditions *PodAdditions `json:"podAdditions,omitempty"`
	// Machine name to exposed endpoint map
	ExposedEndpoints map[string]ExposedEndpointList `json:"exposedEndpoints,omitempty"`
	// Routing reconcile phase
	Phase WorkspaceRoutingPhase `json:"phase,omitempty"`
}

// Valid phases for workspacerouting
type WorkspaceRoutingPhase string

const (
	RoutingReady     WorkspaceRoutingPhase = "Ready"
	RoutingPreparing WorkspaceRoutingPhase = "Preparing"
	RoutingFailed    WorkspaceRoutingPhase = "Failed"
)

type ExposedEndpoint struct {
	// Name of the exposed endpoint
	Name string `json:"name"`
	// Public URL of the exposed endpoint
	Url string `json:"url"`
	// Attributes of the exposed endpoint
	Attributes map[string]string `json:"attributes"`
}

type EndpointList []devworkspace.Endpoint

type ExposedEndpointList []ExposedEndpoint

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkspaceRouting is the Schema for the workspaceroutings API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=workspaceroutings,scope=Namespaced
type WorkspaceRouting struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkspaceRoutingSpec   `json:"spec,omitempty"`
	Status WorkspaceRoutingStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkspaceRoutingList contains a list of WorkspaceRouting
type WorkspaceRoutingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkspaceRouting `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WorkspaceRouting{}, &WorkspaceRoutingList{})
}
