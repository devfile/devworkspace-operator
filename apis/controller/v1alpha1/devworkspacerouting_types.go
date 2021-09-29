//
// Copyright (c) 2019-2021 Red Hat, Inc.
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
package v1alpha1

import (
	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	devfileAttr "github.com/devfile/api/v2/pkg/attributes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DevWorkspaceRoutingSpec defines the desired state of DevWorkspaceRouting
// +k8s:openapi-gen=true
type DevWorkspaceRoutingSpec struct {
	// Id for the DevWorkspace being routed
	DevWorkspaceId string `json:"devworkspaceId"`
	// Class of the routing: this drives which DevWorkspaceRouting controller will manage this routing
	RoutingClass DevWorkspaceRoutingClass `json:"routingClass,omitempty"`
	// Machines to endpoints map
	Endpoints map[string]EndpointList `json:"endpoints"`
	// Selector that should be used by created services to point to the devworkspace Pod
	PodSelector map[string]string `json:"podSelector"`
}

type DevWorkspaceRoutingClass string

const (
	DevWorkspaceRoutingBasic       DevWorkspaceRoutingClass = "basic"
	DevWorkspaceRoutingCluster     DevWorkspaceRoutingClass = "cluster"
	DevWorkspaceRoutingClusterTLS  DevWorkspaceRoutingClass = "cluster-tls"
	DevWorkspaceRoutingWebTerminal DevWorkspaceRoutingClass = "web-terminal"
)

// DevWorkspaceRoutingStatus defines the observed state of DevWorkspaceRouting
// +k8s:openapi-gen=true
type DevWorkspaceRoutingStatus struct {
	// Additions to main devworkspace deployment
	PodAdditions *PodAdditions `json:"podAdditions,omitempty"`
	// Machine name to exposed endpoint map
	ExposedEndpoints map[string]ExposedEndpointList `json:"exposedEndpoints,omitempty"`
	// Routing reconcile phase
	Phase DevWorkspaceRoutingPhase `json:"phase,omitempty"`
	// Message is a user-readable message explaining the current phase (e.g. reason for failure)
	Message string `json:"message,omitempty"`
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

type EndpointList []dw.Endpoint

type ExposedEndpointList []ExposedEndpoint

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DevWorkspaceRouting is the Schema for the devworkspaceroutings API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=devworkspaceroutings,scope=Namespaced,shortName=dwr
// +kubebuilder:printcolumn:name="DevWorkspace ID",type="string",JSONPath=".spec.devworkspaceId",description="The owner DevWorkspace's unique id"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="The current phase"
// +kubebuilder:printcolumn:name="Info",type="string",JSONPath=".status.message",description="Additional info about DevWorkspaceRouting state"
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
