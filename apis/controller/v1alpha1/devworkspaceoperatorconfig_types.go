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
	"fmt"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	appsv1 "k8s.io/api/apps/v1"
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

type WorkspaceConfig struct {
	// ProjectCloneConfig defines configuration related to the project clone init container
	// that is used to clone git projects into the DevWorkspace.
	ProjectCloneConfig *ProjectCloneConfig `json:"projectClone,omitempty"`
	// ImagePullPolicy defines the imagePullPolicy used for containers in a DevWorkspace
	// For additional information, see Kubernetes documentation for imagePullPolicy. If
	// not specified, the default value of "Always" is used.
	// +kubebuilder:validation:Enum=IfNotPresent;Always;Never
	ImagePullPolicy string `json:"imagePullPolicy,omitempty"`
	// DeploymentStrategy defines the deployment strategy to use to replace existing DevWorkspace pods
	// with new ones. The available deployment stragies are "Recreate" and "RollingUpdate".
	// With the "Recreate" deployment strategy, the existing workspace pod is killed before the new one is created.
	// With the "RollingUpdate" deployment strategy, a new workspace pod is created and the existing workspace pod is deleted
	// only when the new workspace pod is in a ready state.
	// If not specified, the default "Recreate" deployment strategy is used.
	// +kubebuilder:validation:Enum=Recreate;RollingUpdate
	DeploymentStrategy appsv1.DeploymentStrategyType `json:"deploymentStrategy,omitempty"`
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
	// ServiceAccount defines configuration options for the ServiceAccount used for
	// DevWorkspaces.
	ServiceAccount *ServiceAccountConfig `json:"serviceAccount,omitempty"`
	// StorageClassName defines an optional storageClass to use for persistent
	// volume claims created to support DevWorkspaces
	StorageClassName *string `json:"storageClassName,omitempty"`
	// DefaultStorageSize defines an optional struct with fields to specify the sizes of Persistent Volume Claims for storage
	// classes used by DevWorkspaces.
	DefaultStorageSize *StorageSizes `json:"defaultStorageSize,omitempty"`
	// PersistUserHome defines configuration options for persisting the `/home/user/`
	// directory in workspaces.
	PersistUserHome *PersistentHomeConfig `json:"persistUserHome,omitempty"`
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
	// pods created by the DevWorkspace Operator. If set, defined values are merged into the default
	// configuration
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
	// ContainerSecurityContext overrides the default ContainerSecurityContext used for all
	// workspace-related containers created by the DevWorkspace Operator. If set, defined
	// values are merged into the default configuration
	ContainerSecurityContext *corev1.SecurityContext `json:"containerSecurityContext,omitempty"`
	// DefaultTemplate defines an optional DevWorkspace Spec Template which gets applied to the workspace
	// if the workspace's Template Spec Components are not defined. The DefaultTemplate will overwrite the existing
	// Template Spec, with the exception of Projects (if any are defined).
	DefaultTemplate *dw.DevWorkspaceTemplateSpecContent `json:"defaultTemplate,omitempty"`
	// SchedulerName is the name of the pod scheduler for DevWorkspace pods.
	// If not specified, the pod scheduler is set to the default scheduler on the cluster.
	SchedulerName string `json:"schedulerName,omitempty"`
	// DefaultContainerResources defines the resource requirements (memory/cpu limit/request) used for
	// container components that do not define limits or requests. In order to not set a field by default,
	// the value "0" should be used. By default, the memory limit is 128Mi and the memory request is 64Mi.
	// No CPU limit or request is added by default.
	DefaultContainerResources *corev1.ResourceRequirements `json:"defaultContainerResources,omitempty"`
}

type PersistentHomeConfig struct {
	// Determines whether the `/home/user/` directory in workspaces should persist between
	// workspace shutdown and startup.
	// Must be used with the 'per-user'/'common' or 'per-workspace' storage class in order to take effect.
	// Disabled by default.
	Enabled *bool `json:"enabled,omitempty"`
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

type ServiceAccountConfig struct {
	// ServiceAccountName defines a fixed name to be used for all DevWorkspaces. If set, the DevWorkspace
	// Operator will not generate a separate ServiceAccount for each DevWorkspace, and will instead create
	// a ServiceAccount with the specified name in each namespace where DevWorkspaces are created. If specified,
	// the created ServiceAccount will not be removed when DevWorkspaces are deleted and must be cleaned up manually.
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +kubebuilder:validation:MaxLength=63
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
	// Disable creation of DevWorkspace ServiceAccounts by the DevWorkspace Operator. If set to true, the serviceAccountName
	// field must also be set. If ServiceAccount creation is disabled, it is assumed that the specified ServiceAccount already
	// exists in any namespace where a workspace is created. If a suitable ServiceAccount does not exist, starting DevWorkspaces
	// will fail.
	DisableCreation *bool `json:"disableCreation,omitempty"`
	// List of ServiceAccount tokens that will be mounted into workspace pods as projected volumes.
	ServiceAccountTokens []ServiceAccountToken `json:"serviceAccountTokens,omitempty"`
}

type ServiceAccountToken struct {
	// Identifiable name of the ServiceAccount token.
	// If multiple ServiceAccount tokens use the same mount path, a generic name will be used
	// for the projected volume instead.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Path within the workspace container at which the token should be mounted.  Must
	// not contain ':'.
	// +kubebuilder:validation:Required
	MountPath string `json:"mountPath"`
	// Path is the path relative to the mount point of the file to project the
	// token into.
	// +kubebuilder:validation:Required
	Path string `json:"path"`
	// Audience is the intended audience of the token. A recipient of a token
	// must identify itself with an identifier specified in the audience of the
	// token, and otherwise should reject the token. The audience defaults to the
	// identifier of the apiserver.
	// +kubebuilder:validation:Optional
	Audience string `json:"audience,omitempty"`
	// ExpirationSeconds is the requested duration of validity of the service
	// account token. As the token approaches expiration, the kubelet volume
	// plugin will proactively rotate the service account token. The kubelet will
	// start trying to rotate the token if the token is older than 80 percent of
	// its time to live or if the token is older than 24 hours. Defaults to 1 hour
	// and must be at least 10 minutes.
	// +kubebuilder:validation:Minimum=600
	// +kubebuilder:default:=3600
	// +kubebuilder:validation:Optional
	ExpirationSeconds int64 `json:"expirationSeconds,omitempty"`
}

type ProjectCloneConfig struct {
	// Image is the container image to use for cloning projects
	Image string `json:"image,omitempty"`
	// ImagePullPolicy configures the imagePullPolicy for the project clone container.
	// If undefined, the general setting .config.workspace.imagePullPolicy is used instead.
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// Resources defines the resource (cpu, memory) limits and requests for the project
	// clone container. To explicitly not specify a limit or request, define the resource
	// quantity as zero ('0')
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// Env allows defining additional environment variables for the project clone container.
	Env []corev1.EnvVar `json:"env,omitempty"`
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
// +kubebuilder:object:root=true
type DevWorkspaceOperatorConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DevWorkspaceOperatorConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DevWorkspaceOperatorConfig{}, &DevWorkspaceOperatorConfigList{})
}

func (saToken ServiceAccountToken) String() string {
	return fmt.Sprintf("{name: %s, path: %s, mountPath: %s, audience: %s, expirationSeconds %d}", saToken.Name, saToken.Path, saToken.MountPath, saToken.Audience, saToken.ExpirationSeconds)
}
