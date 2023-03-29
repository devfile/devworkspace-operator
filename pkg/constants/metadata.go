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

package constants

// Constants that are used in labels and annotations on DevWorkspace-related resources.
const (
	// DevWorkspaceIDLabel is the label key to store workspace identifier
	DevWorkspaceIDLabel = "controller.devfile.io/devworkspace_id"

	// WorkspaceIdOverrideAnnotation is an annotation that can be applied to DevWorkspaces
	// to override the default DevWorkspace ID assigned by the Operator. Is only respected
	// when a DevWorkspace is created. Once a DevWorkspace has an ID set, it cannot be changed.
	WorkspaceIdOverrideAnnotation = "controller.devfile.io/devworkspace_id_override"

	// DevWorkspaceCreatorLabel is the label key for storing the UID of the user who created the workspace
	DevWorkspaceCreatorLabel = "controller.devfile.io/creator"

	// DevWorkspaceNameLabel is the label key to store workspace name
	DevWorkspaceNameLabel = "controller.devfile.io/devworkspace_name"

	// DevWorkspaceWatchConfigMapLabel marks a configmap so that it is watched by the controller. This label is required on all
	// configmaps that should be seen by the controller
	DevWorkspaceWatchConfigMapLabel = "controller.devfile.io/watch-configmap"

	// DevWorkspaceWatchSecretLabel marks a secret so that it is watched by the controller. This label is required on all
	// secrets that should be seen by the controller
	DevWorkspaceWatchSecretLabel = "controller.devfile.io/watch-secret"

	// DevWorkspaceMountLabel is the label key to store if a configmap, secret, or PVC should be mounted to the devworkspace
	DevWorkspaceMountLabel = "controller.devfile.io/mount-to-devworkspace"

	// DevWorkspaceMountPathAnnotation is the annotation key to store the mount path for the secret or configmap.
	// If no mount path is provided, configmaps will be mounted at /etc/config/<configmap-name>, secrets will
	// be mounted at /etc/secret/<secret-name>, and persistent volume claims will be mounted to /tmp/<claim-name>
	DevWorkspaceMountPathAnnotation = "controller.devfile.io/mount-path"

	// DevWorkspaceMountAsAnnotation is the annotation key to configure the way how configmaps or secrets should be mounted.
	// Supported options:
	// - "env" - mount as environment variables
	// - "file" - mount as files within the mount path
	// - "subpath" - mount keys as subpath volume mounts within the mount path
	// When a configmap or secret is mounted via "file", the keys within the configmap/secret are mounted as files
	// within a directory, erasing all contents of the directory. Mounting via "subpath" leaves existing files in the
	// mount directory changed, but prevents on-cluster changes to the configmap/secret propagating to the container
	// until it is restarted.
	// If mountAs is not provided, the default behaviour will be to mount as a file.
	DevWorkspaceMountAsAnnotation = "controller.devfile.io/mount-as"

	// DevWorkspaceMountAccessModeAnnotation is an annotation key used to configure the access mode for configmaps and
	// secrets mounted using the 'controller.devfile.io/mount-to-devworkspace' annotation. The access mode annotation
	// can either be specified as a decimal (e.g. '416') or as an octal by prefixing the number with zero (e.g. '0640')
	DevWorkspaceMountAccessModeAnnotation = "controller.devfile.io/mount-access-mode"

	// DevWorkspaceGitCredentialLabel is the label key to specify if the secret is a git credential. All secrets who
	// specify this label in a namespace will consolidate into one secret before mounting into a devworkspace.
	// Only secret data with the credentials key will be used and credentials must be the base64 encoded version
	//	of https://{USERNAME}:{PERSONAL_ACCESS_TOKEN}@{GIT_WEBSITE}
	// E.g. echo -n "https://{USERNAME}:{PERSONAL_ACCESS_TOKEN}@{GIT_WEBSITE}" | base64
	// see https://git-scm.com/docs/git-credential-store#_storage_format for more details
	DevWorkspaceGitCredentialLabel = "controller.devfile.io/git-credential"

	// DevWorkspaceGitTLSLabel is the label key to specify if the configmap is credentials for accessing a git server.
	// Configmap must contain the following data:
	// certificate: the certificate used to access the git server in Base64 ASCII
	// You can also optionally define the git host.
	// host: the url of the git server
	// If the git host is not defined then the certificate will be used for all http repositories.
	DevWorkspaceGitTLSLabel = "controller.devfile.io/git-tls-credential"

	// GitCredentialsConfigMapName is the name used for the configmap that stores the Git configuration for workspaces
	// in a given namespace. It is used when e.g. adding Git credentials via secret
	GitCredentialsConfigMapName = "devworkspace-gitconfig"

	// GitCredentialsMergedSecretName is the name for the merged Git credentials secret that is mounted to workspaces
	// when Git credentials are defined. This secret combines the values of any secrets labelled
	// "controller.devfile.io/git-credential"
	GitCredentialsMergedSecretName = "devworkspace-merged-git-credentials"

	// DevWorkspaceMountAsEnv is the annotation value for DevWorkspaceMountAsAnnotation to mount the resource as environment variables
	// via envFrom
	DevWorkspaceMountAsEnv = "env"
	// DevWorkspaceMountAsFile is the annotation value for DevWorkspaceMountAsAnnotation to mount the resource as files
	DevWorkspaceMountAsFile = "file"
	// DevWorkspaceMountAsSubpath is the annotation value for DevWorkspaceMountAsAnnotation to mount the resource as files using subpath
	// mounts
	DevWorkspaceMountAsSubpath = "subpath"

	// DevWorkspaceMountReadyOnlyAnnotation is an annotation to configure whether a mounted volume is as read-write or
	// as read-only. If "true", the volume is mounted as read-only. PersistentVolumeClaims are by default mounted
	// read-write. Automounted configmaps and secrets are always mounted read-only and this annotation is ignored.
	DevWorkspaceMountReadyOnlyAnnotation = "controller.devfile.io/read-only"

	// DevWorkspaceRestrictedAccessAnnotation marks the intention that devworkspace access is restricted to only the creator; setting this
	// annotation will cause devworkspace start to fail if webhooks are disabled.
	// Operator also propagates it to the devworkspace-related objects to perform authorization.
	DevWorkspaceRestrictedAccessAnnotation = "controller.devfile.io/restricted-access"

	// DevWorkspaceStartedStatusAnnotation is applied to subresources of DevWorkspaces to indicate the owning object's
	// .spec.started value. This annotation is applied to DevWorkspaceRoutings to trigger reconciles when a DevWorkspace
	// is started or stopped.
	DevWorkspaceStartedStatusAnnotation = "controller.devfile.io/devworkspace-started"

	// DevWorkspaceStopReasonAnnotation marks the reason why the devworkspace was stopped; when a devworkspace is restarted
	// this annotation will be cleared
	DevWorkspaceStopReasonAnnotation = "controller.devfile.io/stopped-by"

	// DevWorkspaceDebugStartAnnotation enables debugging workspace startup if set to "true". If a workspace with this annotation
	// fails to start (i.e. enters the "Failed" phase), its deployment will not be scaled down in order to allow viewing logs, etc.
	DevWorkspaceDebugStartAnnotation = "controller.devfile.io/debug-start"

	// WebhookRestartedAtAnnotation holds the the time (unixnano) of when the webhook server was forced to restart by controller
	WebhookRestartedAtAnnotation = "controller.devfile.io/restarted-at"

	// DevWorkspaceStartedAtAnnotation holds the the time (unixnano) of when the devworkspace was started
	DevWorkspaceStartedAtAnnotation = "controller.devfile.io/started-at"

	// RoutingAnnotationInfix is the infix of the annotations of DevWorkspace that are passed down as annotation to the DevWorkspaceRouting objects.
	// The full annotation name is supposed to be "<routingClass>.routing.controller.devfile.io/<anything>"
	RoutingAnnotationInfix = ".routing.controller.devfile.io/"

	// DevWorkspaceEndpointNameAnnotation is the annotation key for storing an endpoint's name from the devfile representation
	DevWorkspaceEndpointNameAnnotation = "controller.devfile.io/endpoint_name"

	// DevWorkspaceDiscoverableServiceAnnotation marks a service in a devworkspace as created for a discoverable endpoint,
	// as opposed to a service created to support the devworkspace itself.
	DevWorkspaceDiscoverableServiceAnnotation = "controller.devfile.io/discoverable-service"

	// DevWorkspacePullSecretLabel marks the intention that this secret should be used as a pull secret for devworkspaces within namespace
	// Only secrets with 'true' value will be mount as pull secret
	// Should be assigned to secrets with type docker config types (kubernetes.io/dockercfg and kubernetes.io/dockerconfigjson)
	DevWorkspacePullSecretLabel = "controller.devfile.io/devworkspace_pullsecret"

	// NamespacedConfigLabelKey is a label applied to configmaps to mark them as a configuration for all DevWorkspaces in
	// the current namespace.
	NamespacedConfigLabelKey = "controller.devfile.io/namespaced-config"

	// NamespacePodTolerationsAnnotation is an annotation applied to a namespace to configure pod tolerations for all workspaces
	// in that namespace. Value should be json-encoded []corev1.Toleration struct.
	NamespacePodTolerationsAnnotation = "controller.devfile.io/pod-tolerations"

	// NamespaceNodeSelectorAnnotation is an annotation applied to a namespace to configure the node selector for all workspaces
	// in that namespace. Value should be json-encoded map[string]string
	NamespaceNodeSelectorAnnotation = "controller.devfile.io/node-selector"
)
