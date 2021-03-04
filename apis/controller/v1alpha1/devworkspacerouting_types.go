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

package v1alpha1

import (
	devworkspace "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	devfileAttr "github.com/devfile/api/v2/pkg/attributes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DevWorkspaceRoutingSpec defines the desired state of DevWorkspaceRouting
// +k8s:openapi-gen=true
type DevWorkspaceRoutingSpec struct {
	// WorkspaceId for the workspace being routed
	WorkspaceId string `json:"workspaceId"`
	// Class of the routing: this drives which Workspace Routing controller will manage this routing
	RoutingClass DevWorkspaceRoutingClass `json:"routingClass,omitempty"`
	// Routing suffix for cluster
	RoutingSuffix string `json:"routingSuffix"`
	// Machines to endpoints map
	Endpoints map[string]EndpointList `json:"endpoints"`
	// Selector that should be used by created services to point to the workspace Pod
	PodSelector map[string]string `json:"podSelector"`
}

type DevWorkspaceRoutingClass string

const (
	DevWorkspaceRoutingBasic          DevWorkspaceRoutingClass = "basic"
	DevWorkspaceRoutingOpenShiftOauth DevWorkspaceRoutingClass = "openshift-oauth"
	DevWorkspaceRoutingCluster        DevWorkspaceRoutingClass = "cluster"
	DevWorkspaceRoutingClusterTLS     DevWorkspaceRoutingClass = "cluster-tls"
	DevWorkspaceRoutingWebTerminal    DevWorkspaceRoutingClass = "web-terminal"
)

// DevWorkspaceRoutingStatus defines the observed state of DevWorkspaceRouting
// +k8s:openapi-gen=true
type DevWorkspaceRoutingStatus struct {
	// Additions to main workspace deployment
	PodAdditions *PodAdditions `json:"podAdditions,omitempty"`
	// Machine name to exposed endpoint map
	ExposedEndpoints map[string]ExposedEndpointList `json:"exposedEndpoints,omitempty"`
	// Routing reconcile phase
	Phase DevWorkspaceRoutingPhase `json:"phase,omitempty"`
}

// Valid phases for devworkspacerouting
type DevWorkspaceRoutingPhase string

const (
	RoutingReady     DevWorkspaceRoutingPhase = "Ready"
	RoutingPreparing DevWorkspaceRoutingPhase = "Preparing"
	RoutingFailed    DevWorkspaceRoutingPhase = "Failed"
)

type ExposedEndpoint struct {
	// Name of the exposed endpoint
	Name string `json:"name"`
	// Public URL of the exposed endpoint
	Url string `json:"url"`
	// Attributes of the exposed endpoint
	// +optional
	Attributes devfileAttr.Attributes `json:"attributes,omitempty"`
}

type EndpointList []devworkspace.Endpoint

type ExposedEndpointList []ExposedEndpoint

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DevWorkspaceRouting is the Schema for the devworkspaceroutings API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=devworkspaceroutings,scope=Namespaced,shortName=dwr
type DevWorkspaceRouting struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DevWorkspaceRoutingSpec   `json:"spec,omitempty"`
	Status DevWorkspaceRoutingStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DevWorkspaceRoutingList contains a list of DevWorkspaceRouting
type DevWorkspaceRoutingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DevWorkspaceRouting `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DevWorkspaceRouting{}, &DevWorkspaceRoutingList{})
}
