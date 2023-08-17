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

package v1alpha1

import (
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
	RoutingStopped   DevWorkspaceRoutingPhase = "Stopped"
)

type ExposedEndpoint struct {
	// Name of the exposed endpoint
	Name string `json:"name"`
	// Public URL of the exposed endpoint
	Url string `json:"url"`
	// Attributes of the exposed endpoint
	// +optional
	Attributes Attributes `json:"attributes,omitempty"`
}

type EndpointList []Endpoint

type ExposedEndpointList []ExposedEndpoint

// EndpointExposure describes the way an endpoint is exposed on the network.
// Only one of the following exposures may be specified: public, internal, none.
// +kubebuilder:validation:Enum=public;internal;none
type EndpointExposure string

const (
	// Endpoint will be exposed on the public network, typically through
	// a K8S ingress or an OpenShift route
	PublicEndpointExposure EndpointExposure = "public"
	// Endpoint will be exposed internally outside of the main devworkspace POD,
	// typically by K8S services, to be consumed by other elements running
	// on the same cloud internal network.
	InternalEndpointExposure EndpointExposure = "internal"
	// Endpoint will not be exposed and will only be accessible
	// inside the main devworkspace POD, on a local address.
	NoneEndpointExposure EndpointExposure = "none"
)

// EndpointProtocol defines the application and transport protocols of the traffic that will go through this endpoint.
// Only one of the following protocols may be specified: http, ws, tcp, udp.
// +kubebuilder:validation:Enum=http;https;ws;wss;tcp;udp
type EndpointProtocol string

// Attributes provides a way to add a map of arbitrary YAML/JSON
// objects.
// +kubebuilder:validation:Type=object
// +kubebuilder:validation:XPreserveUnknownFields
type Attributes map[string]apiext.JSON

type Endpoint struct {
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name"`

	TargetPort int `json:"targetPort"`

	// Describes how the endpoint should be exposed on the network.
	//
	// - `public` means that the endpoint will be exposed on the public network, typically through
	// a K8S ingress or an OpenShift route.
	//
	// - `internal` means that the endpoint will be exposed internally outside of the main devworkspace POD,
	// typically by K8S services, to be consumed by other elements running
	// on the same cloud internal network.
	//
	// - `none` means that the endpoint will not be exposed and will only be accessible
	// inside the main devworkspace POD, on a local address.
	//
	// Default value is `public`
	// +optional
	// +kubebuilder:default=public
	Exposure EndpointExposure `json:"exposure,omitempty"`

	// Describes the application and transport protocols of the traffic that will go through this endpoint.
	//
	// - `http`: Endpoint will have `http` traffic, typically on a TCP connection.
	// It will be automaticaly promoted to `https` when the `secure` field is set to `true`.
	//
	// - `https`: Endpoint will have `https` traffic, typically on a TCP connection.
	//
	// - `ws`: Endpoint will have `ws` traffic, typically on a TCP connection.
	// It will be automaticaly promoted to `wss` when the `secure` field is set to `true`.
	//
	// - `wss`: Endpoint will have `wss` traffic, typically on a TCP connection.
	//
	// - `tcp`: Endpoint will have traffic on a TCP connection, without specifying an application protocol.
	//
	// - `udp`: Endpoint will have traffic on an UDP connection, without specifying an application protocol.
	//
	// Default value is `http`
	// +optional
	// +kubebuilder:default=http
	Protocol EndpointProtocol `json:"protocol,omitempty"`

	// Describes whether the endpoint should be secured and protected by some
	// authentication process. This requires a protocol of `https` or `wss`.
	// +optional
	// +devfile:default:value=false
	Secure bool `json:"secure,omitempty"`

	// Path of the endpoint URL
	// +optional
	Path string `json:"path,omitempty"`

	// Map of implementation-dependant string-based free-form attributes.
	//
	// Examples of Che-specific attributes:
	//
	// - cookiesAuthEnabled: "true" / "false",
	//
	// - type: "terminal" / "ide" / "ide-dev",
	// +optional
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Attributes Attributes `json:"attributes,omitempty"`
}

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
