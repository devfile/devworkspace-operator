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

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkspaceConditionType is a valid value for WorkspaceCondition.Type
type WorkspaceConditionType string

// These are valid conditions of pod.
const (
	// WorkspaceConditionReady means the main workspace features (IDE, terminal, etc ...) are ready to use
	WorkspaceConditionReady WorkspaceConditionType = "Ready"
	// WorkspaceConditionInitialized means that all init containers involved in workspace plugin initialization
	// in the pod have started successfully.
	WorkspaceConditionInitialized WorkspaceConditionType = "Initialized"
	// WorkspaceScheduled represents status of the scheduling process for the workspace underlying resources.
	WorkspaceConditionScheduled WorkspaceConditionType = "Scheduled"
	// WorkspaceStopped means the workspace is stopped
	WorkspaceConditionStopped WorkspaceConditionType = "Stopped"

	// Reason the explains why all the conditions might be false. Not ready nor stopped
	WorkspaceConditionStoppingReason = "CleaningResourcesToStop"

	// Reason the explains that workspace could not start due to a reconcile failure
	WorkspaceConditionReconcileFailureReason = "ReconcileFailure"
)

// WorkspaceCondition contains details for the current condition of this workspace.
type WorkspaceCondition struct {
	// Type is the type of the condition.
	Type WorkspaceConditionType `json:"type"`
	// Status is the status of the condition.
	// Can be True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// Unique, one-word, CamelCase reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Human-readable message indicating details about last transition.
	Message string `json:"message,omitempty"`
}

// WorkspacePhase is a label for the condition of a workspace at the current time.
type WorkspacePhase string

// These are the valid statuses of pods.
const (
	WorkspacePhaseStopped  WorkspacePhase = "Stopped"
	WorkspacePhaseStarting WorkspacePhase = "Starting"
	WorkspacePhaseStopping WorkspacePhase = "Stopping"
	WorkspacePhaseRunning  WorkspacePhase = "Running"
	WorkspacePhaseFailed   WorkspacePhase = "Failed"
)

type MembersStatus struct {
	// Ready are the workspace Pods that are ready
	// The member names are based on the workspace pod
	// deployment names
	Ready []string `json:"ready,omitempty"`
	// Unready are the workspace Pods that are not
	// ready to serve requests
	Unready []string `json:"unready,omitempty"`
}

// WorkspaceStatus defines the observed state of Workspace
// +k8s:openapi-gen=false
type WorkspaceStatus struct {
	// Id of the workspace
	WorkspaceId string `json:"workspaceId"`
	// Workspace status
	Phase WorkspacePhase `json:"phase"`
	// Condition keeps track of all cluster conditions, if they exist.
	Conditions []WorkspaceCondition `json:"conditions,omitempty"`
	// Members are the Workspace pods
	Members MembersStatus `json:"members"`
	// URL at which the Editor can be joined
	IdeUrl string `json:"ideUrl,omitempty"`
	// AdditionalInfo
	AdditionalInfo map[string]string `json:"additionalFields,omitempty"`
}
