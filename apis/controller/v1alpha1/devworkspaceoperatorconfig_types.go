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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OperatorConfiguration defines configuration options for the DevWorkspace
// Operator.
type OperatorConfiguration struct {
	// Routing defines configuration options related to DevWorkspace networking
	Routing *RoutingConfig `json:"routing,omitempty"`
	// Workspace defines configuration options related to how DevWorkspaces are
	// managed
	Workspace *WorkspaceConfig `json:"workspace,omitempty"`
	// EnableExperimentalFeatures turns on in-development features of the controller.
	// This option should generally not be enabled, as any capabilites are subject
	// to removal without notice.
	EnableExperimentalFeatures *bool `json:"enableExperimentalFeatures,omitempty"`
}

type RoutingConfig struct {
	// DefaultRoutingClass specifies the routingClass to be used when a DevWorkspace
	// specifies an empty `.spec.routingClass`. Supported routingClasses can be defined
	// in other controllers. If not specified, the default value of "basic" is used.
	DefaultRoutingClass string `json:"defaultRoutingClass,omitempty"`
	// ClusterHostSuffix is the hostname suffix to be used for DevWorkspace endpoints.
	// On OpenShift, the DevWorkspace Operator will attempt to determine the appropriate
	// value automatically. Must be specified on Kubernetes.
	ClusterHostSuffix string `json:"clusterHostSuffix,omitempty"`
}

type WorkspaceConfig struct {
	// ImagePullPolicy defines the imagePullPolicy used for containers in a DevWorkspace
	// For additional information, see Kubernetes documentation for imagePullPolicy. If
	// not specified, the default value of "Always" is used.
	// +kubebuilder:validation:Enum=IfNotPresent;Always;Never
	ImagePullPolicy string `json:"imagePullPolicy,omitempty"`
	// PVCName defines the name used for the persistent volume claim created
	// to support workspace storage when the 'common' storage class is used.
	// If not specified, the default value of `claim-devworkspace` is used.
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +kubebuilder:validation:MaxLength=63
	PVCName string `json:"pvcName,omitempty"`
	// StorageClassName defines and optional storageClass to use for persistent
	// volume claims created to support DevWorkspaces
	StorageClassName *string `json:"storageClassName,omitempty"`
	// IdleTimeout determines how long a workspace should sit idle before being
	// automatically scaled down. Proper functionality of this configuration property
	// requires support in the workspace being started. If not specified, the default
	// value of "15m" is used.
	IdleTimeout string `json:"idleTimeout,omitempty"`
	// IgnoredUnrecoverableEvents defines a list of Kubernetes event names that should
	// be ignored when deciding to fail a DevWorkspace startup. This option should be used
	// if a transient cluster issue is triggering false-positives (for example, if
	// the cluster occasionally encounters FailedScheduling events). Events listed
	// here will not trigger DevWorkspace failures.
	IgnoredUnrecoverableEvents []string `json:"ignoredUnrecoverableEvents,omitempty"`
}

// DevWorkspaceOperatorConfig is the Schema for the devworkspaceoperatorconfigs API
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=devworkspaceoperatorconfigs,scope=Namespaced,shortName=dwoc
type DevWorkspaceOperatorConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Config *OperatorConfiguration `json:"config,omitempty"`
}

// DevWorkspaceOperatorConfigList contains a list of DevWorkspaceOperatorConfig
//+kubebuilder:object:root=true
type DevWorkspaceOperatorConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DevWorkspaceOperatorConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DevWorkspaceOperatorConfig{}, &DevWorkspaceOperatorConfigList{})
}
