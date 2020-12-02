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
	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ComponentSpec defines the desired state of Component
// +k8s:openapi-gen=true
type WorkspaceComponentSpec struct {
	// Id of workspace that contains this component
	WorkspaceId string `json:"workspaceId"`
	// List of devfile components to be processed by this component
	Components []devworkspace.Component `json:"components"`
	// Commands from devfile, to be matched to components
	Commands []devworkspace.Command `json:"commands,omitempty"`
	// Events
	Events *devworkspace.Events `json:"events,omitempty"`
}

// ComponentStatus defines the observed state of Component
// +k8s:openapi-gen=true
type WorkspaceComponentStatus struct {
	// Ready indicates whether the component has finished processing its spec
	Ready bool `json:"ready"`
	// Failed indicates that an unresolvable problem prevents this component
	// from being ready.
	Failed bool `json:"failed,omitempty"`
	// Message stores additional context about the Component's current state (e.g.
	// reason for failure)
	Message string `json:"message,omitempty"`
	// ComponentDescriptions of processed components from spec
	ComponentDescriptions []ComponentDescription `json:"componentDescriptions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Component is the Schema for the components API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=components,scope=Namespaced
type Component struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkspaceComponentSpec   `json:"spec,omitempty"`
	Status WorkspaceComponentStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ComponentList contains a list of Component
type ComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Component `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Component{}, &ComponentList{})
}
