package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkspaceRouting is the Schema for the workspaceroutings API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
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

// WorkspaceRoutingSpec defines the desired state of WorkspaceRouting
// +k8s:openapi-gen=true
type WorkspaceRoutingSpec struct {
	// Class of the routing: this drives which Workspace Routing controller will manage this routing
	RoutingClass string `json:"routingClass"`

	//Should the workspace be exposed ?
	Exposed bool `json:"exposed"`

	// ingress global domain (corresponds to the OpenShift route suffix)
	IngressGlobalDomain string `json:"ingressGlobalDomain"`

	// Selector that should be used by created services to point to the workspace Pod
	WorkspacePodSelector map[string]string `json:"workspacePodSelector"`

	//Services by machine name
	Services map[string]ServiceDescription `json:"services"`
}

type ServiceDescription struct {
	// Service name of the machine-related service
	ServiceName string `json:"serviceName"`
	// Endpoints that correspond to this machine-related service
	Endpoints []Endpoint `json:"endpoints"`
}

type ExposedEndpoint struct {
	// Name of the exposed endpoint
	Name string `json:"name"`
	// Url of the exposed endpoint
	Url string `json:"url"`
	// Attributes of the exposed endpoint
	Attributes map[EndpointAttribute]string `json:"attributes,omitempty"`
}

// WorkspaceRoutingPhase is a label for the condition of a workspace routing at the current time.
type WorkspaceRoutingPhase string

// These are the valid statuses of pods.
const (
	WorkspaceRoutingExposing WorkspaceRoutingPhase = "Exposing"
	WorkspaceRoutingExposed  WorkspaceRoutingPhase = "Exposed"
	WorkspaceRoutingReady    WorkspaceRoutingPhase = "Ready"
	WorkspaceRoutingHidden   WorkspaceRoutingPhase = "Hidden"
	WorkspaceRoutingHiding   WorkspaceRoutingPhase = "Hiding"
	WorkspaceRoutingFailed   WorkspaceRoutingPhase = "Failed"
)

// WorkspaceRoutingStatus defines the observed state of WorkspaceRouting
// +k8s:openapi-gen=true
type WorkspaceRoutingStatus struct {
	// Workspace Routing status
	Phase            WorkspaceRoutingPhase          `json:"phase,omitempty"`
	ExposedEndpoints map[string]ExposedEndpointList `json:"exposedEndpoints,omitempty"`
}

// +k8s:openapi-gen=true
type ExposedEndpointList []ExposedEndpoint

func init() {
	SchemeBuilder.Register(&WorkspaceRouting{}, &WorkspaceRoutingList{})
}
