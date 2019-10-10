package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// WorkspaceExposureSpec defines the desired state of WorkspaceExposure
// +k8s:openapi-gen=true
type WorkspaceExposureSpec struct {
	// Class of the exposure: this drives which Workspace exposer controller will manage this exposure
	ExposureClass    string `json:"exposureClass"`
	// Should the workspace be exposed ?
	Exposed          bool `json:"exposed"`
	// ingress global domain (corresponds to the Openshift route suffix)
	IngressGlobalDomain string `json:"ingressGlobalDomain"`
	// Services by machine name
	Services map[string]ServiceDescription `json:"services,omitEmpty"`
}

type ServiceDescription struct {
	// Service name of the machine-related service
	ServiceName          string `json:"serviceName,omitEmpty"`
	// Endpoints that correspond to this machine-related service
	Endpoints            []Endpoint `json:"endpoints,omitEmpty"`
}

type ExposedEndpoint struct {
	// Url of the exposed endpoint
	Url             string `json:"url,omitEmpty"`
	// Attributes of the exposed endpoint
	Attributes      EndpointAttributes `json:"attributes,omitEmpty"`
}

// WorkspaceExposurePhase is a label for the condition of a workspace exposure at the current time.
type WorkspaceExposurePhase string

// These are the valid statuses of pods.
const (
	WorkspaceExposureProcessing WorkspaceExposurePhase = "Processing"
	WorkspaceExposureExposed WorkspaceExposurePhase = "Exposed"
	WorkspaceExposureHidden WorkspaceExposurePhase = "Hidden"
	WorkspaceExposureFailed   WorkspaceExposurePhase = "Failed"
)

// WorkspaceExposureStatus defines the observed state of WorkspaceExposure
// +k8s:openapi-gen=true
type WorkspaceExposureStatus struct {
	// Workspace Exposure status
	Phase WorkspaceExposurePhase `json:"phase,omitEmpty"`
	ExposedEndpoints []ExposedEndpoint `json:"exposedEndpoints,omitEmpty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkspaceExposure is the Schema for the workspaceexposures API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type WorkspaceExposure struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkspaceExposureSpec   `json:"spec,omitempty"`
	Status WorkspaceExposureStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkspaceExposureList contains a list of WorkspaceExposure
type WorkspaceExposureList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkspaceExposure `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WorkspaceExposure{}, &WorkspaceExposureList{})
}
