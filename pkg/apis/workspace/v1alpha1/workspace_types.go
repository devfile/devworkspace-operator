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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WorkspaceSpec defines the desired state of Workspace
// +k8s:openapi-gen=true
type WorkspaceSpec struct {
	// Whether the workspace should be started or stopped
	Started bool `json:"started"`
	// Routing class the defines how the workspace will be exposed to the external network
	RoutingClass WorkspaceRoutingClass `json:"routingClass,omitempty"`
	// Workspace Structure defined in the Devfile format syntax.
	// For more details see the Che 7 documentation: https://www.eclipse.org/che/docs/che-7/making-a-workspace-portable-using-a-devfile/
	Devfile DevfileSpec `json:"devfile"`
}

// WorkspaceStatus defines the observed state of Workspace
// +k8s:openapi-gen=true
type WorkspaceStatus struct {
	WorkspaceId string         `json:"workspaceId"`
	Phase       WorkspacePhase `json:"phase,omitempty"`
	IdeUrl      string         `json:"ideUrl"`
	// Conditions represent the latest available observations of an object's state
	// +listType=map
	Condition []WorkspaceCondition `json:"condition,omitempty"`

	// TODO: This could potentially be handled via configmap more cleanly
	AdditionalFields WorkspaceStatusAdditionalFields `json:"additionalFields,omitempty"`
}

type WorkspaceStatusAdditionalFields struct {
	Runtime string `json:"org.eclipse.che.workspace/runtime"`
}

// WorkspaceCondition contains details for the current condition of this workspace.
type WorkspaceCondition struct {
	// Type is the type of the condition.
	Type WorkspaceConditionType `json:"type"`
	// Phase is the status of the condition.
	// Can be True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// Unique, one-word, CamelCase reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Human-readable message indicating details about last transition.
	Message string `json:"message,omitempty"`
}

type WorkspacePhase string

// Valid workspace Statuses
const (
	WorkspaceStatusStarting WorkspacePhase = "Starting"
	WorkspaceStatusReady    WorkspacePhase = "Ready"
	WorkspaceStatusStopped  WorkspacePhase = "Stopped"
	WorkspaceStatusFailed   WorkspacePhase = "Failed"
)

// Types of conditions reported by workspace
type WorkspaceConditionType string

const (
	WorkspaceComponentsReady     WorkspaceConditionType = "ComponentsReady"
	WorkspaceRoutingReady        WorkspaceConditionType = "RoutingReady"
	WorkspaceServiceAccountReady WorkspaceConditionType = "ServiceAccountReady"
	WorkspaceDeploymentReady     WorkspaceConditionType = "DeploymentReady"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Workspace is the Schema for the workspaces API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=workspaces,scope=Namespaced
// +kubebuilder:printcolumn:name="Workspace ID",type="string",JSONPath=".status.workspaceId",description="The workspace's unique id"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="The current workspace startup phase"
// +kubebuilder:printcolumn:name="URL",type="string",JSONPath=".status.ideUrl",description="Url endpoint for accessing workspace"
type Workspace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkspaceSpec   `json:"spec,omitempty"`
	Status WorkspaceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkspaceList contains a list of Workspace
type WorkspaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workspace `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Workspace{}, &WorkspaceList{})
}
