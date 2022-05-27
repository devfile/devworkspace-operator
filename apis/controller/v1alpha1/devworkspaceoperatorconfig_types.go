//
// Copyright (c) 2019-2022 Red Hat, Inc.
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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
	// ProxyConfig defines the proxy settings that should be used for all DevWorkspaces.
	// These values are propagated to workspace containers as environment variables.
	//
	// On OpenShift, the operator automatically reads values from the "cluster" proxies.config.openshift.io
	// object and this value only needs to be set to override those defaults. Values for httpProxy
	// and httpsProxy override the cluster configuration directly. Entries for noProxy are merged
	// with the noProxy values in the cluster configuration.
	//
	// Changes to the proxy configuration are detected by the DevWorkspace Operator and propagated to
	// DevWorkspaces. However, changing the proxy configuration for the DevWorkspace Operator itself
	// requires restarting the controller deployment.
	ProxyConfig *Proxy `json:"proxyConfig,omitempty"`
}

type Proxy struct {
	// HttpProxy is the URL of the proxy for HTTP requests, in the format http://USERNAME:PASSWORD@SERVER:PORT/
	HttpProxy string `json:"httpProxy,omitempty"`
	// HttpsProxy is the URL of the proxy for HTTPS requests, in the format http://USERNAME:PASSWORD@SERVER:PORT/
	HttpsProxy string `json:"httpsProxy,omitempty"`
	// NoProxy is a comma-separated list of hostnames and/or CIDRs for which the proxy should not be used. Ignored
	// when HttpProxy and HttpsProxy are unset
	NoProxy string `json:"noProxy,omitempty"`
}

type StorageSizes struct {
	// The default Persistent Volume Claim size for the "common" storage class.
	// Note that the "async" storage class also uses the PVC size set for the "common" storage class.
	// If not specified, the "common" and "async" Persistent Volume Claim sizes are set to 10Gi
	Common *resource.Quantity `json:"common,omitempty"`
	// The default Persistent Volume Claim size for the "per-workspace" storage class.
	// If not specified, the "per-workspace" Persistent Volume Claim size is set to 5Gi
	PerWorkspace *resource.Quantity `json:"perWorkspace,omitempty"`
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
	// Note that changing this configuration value after workspaces have been
	// created will disconnect all existing workspaces from the previously-used
	// persistent volume claim, and will require manual removal of the old PVCs
	// in the cluster.
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +kubebuilder:validation:MaxLength=63
	PVCName string `json:"pvcName,omitempty"`
	// StorageClassName defines an optional storageClass to use for persistent
	// volume claims created to support DevWorkspaces
	StorageClassName *string `json:"storageClassName,omitempty"`
	// DefaultStorageSize defines an optional struct with fields to specify the sizes of Persistent Volume Claims for storage
	// classes used by DevWorkspaces.
	DefaultStorageSize *StorageSizes `json:"defaultStorageSize,omitempty"`
	// IdleTimeout determines how long a workspace should sit idle before being
	// automatically scaled down. Proper functionality of this configuration property
	// requires support in the workspace being started. If not specified, the default
	// value of "15m" is used.
	IdleTimeout string `json:"idleTimeout,omitempty"`
	// ProgressTimeout determines the maximum duration a DevWorkspace can be in
	// a "Starting" or "Failing" phase without progressing before it is automatically failed.
	// Duration should be specified in a format parseable by Go's time package, e.g.
	// "15m", "20s", "1h30m", etc. If not specified, the default value of "5m" is used.
	ProgressTimeout string `json:"progressTimeout,omitempty"`
	// IgnoredUnrecoverableEvents defines a list of Kubernetes event names that should
	// be ignored when deciding to fail a DevWorkspace startup. This option should be used
	// if a transient cluster issue is triggering false-positives (for example, if
	// the cluster occasionally encounters FailedScheduling events). Events listed
	// here will not trigger DevWorkspace failures.
	IgnoredUnrecoverableEvents []string `json:"ignoredUnrecoverableEvents,omitempty"`
	// CleanupOnStop governs how the Operator handles stopped DevWorkspaces. If set to
	// true, additional resources associated with a DevWorkspace (e.g. services, deployments,
	// configmaps, etc.) will be removed from the cluster when a DevWorkspace has
	// .spec.started = false. If set to false, resources will be scaled down (e.g. deployments
	// but the objects will be left on the cluster). The default value is false.
	CleanupOnStop *bool `json:"cleanupOnStop,omitempty"`
	// PodSecurityContext overrides the default PodSecurityContext used for all workspace-related
	// pods created by the DevWorkspace Operator when running on Kubernetes. On OpenShift, this
	// configuration option is ignored. If set, the entire pod security context is overridden;
	// values are not merged.
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
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
