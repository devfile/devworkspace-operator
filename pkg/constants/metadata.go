//
// Copyright (c) 2019-2021 Red Hat, Inc.
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

// Constants that are used in labels and annotation on DevWorkspace-related resources.
const (
	// DevWorkspaceIDLabel is the label key to store workspace identifier
	DevWorkspaceIDLabel = "controller.devfile.io/devworkspace_id"

	// DevWorkspaceCreatorLabel is the label key for storing the UID of the user who created the workspace
	DevWorkspaceCreatorLabel = "controller.devfile.io/creator"

	// DevWorkspaceNameLabel is the label key to store workspace name
	DevWorkspaceNameLabel = "controller.devfile.io/devworkspace_name"

	// DevWorkspaceMountLabel is the label key to store if a configmap or secret should be mounted to the devworkspace
	DevWorkspaceMountLabel = "controller.devfile.io/mount-to-devworkspace"

	// DevWorkspaceGitCredentialLabel is the label key to specify if the secret is a git credential. All secrets who
	// specify this label in a namespace will consolidate into one secret before mounting into a devworkspace.
	// Only secret data with the credentials key will be used and credentials must be the base64 encoded version
	//	of https://{USERNAME}:{PERSONAL_ACCESS_TOKEN}@{GIT_WEBSITE}
	// E.g. echo -n "https://{USERNAME}:{PERSONAL_ACCESS_TOKEN}@{GIT_WEBSITE}" | base64
	// see https://git-scm.com/docs/git-credential-store#_storage_format for more details
	DevWorkspaceGitCredentialLabel = "controller.devfile.io/git-credential"

	// DevWorkspaceMountPathAnnotation is the annotation key to store the mount path for the secret or configmap.
	// If no mount path is provided, configmaps will be mounted at /etc/config/<configmap-name>, secrets will
	// be mounted at /etc/secret/<secret-name>, and persistent volume claims will be mounted to /tmp/<claim-name>
	DevWorkspaceMountPathAnnotation = "controller.devfile.io/mount-path"

	// DevWorkspaceMountAsAnnotation is the annotation key to configure the way how configmaps or secrets should be mounted.
	// Supported options:
	// - "env" - mount as environment variables
	// - "file" - mount as a file
	// If mountAs is not provided, the default behaviour will be to mount as a file.
	DevWorkspaceMountAsAnnotation = "controller.devfile.io/mount-as"

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

	// RoutingAnnotationInfix is the infix of the annotations of DevWorkspace that are passed down as annotation to the DevWorkspaceRouting objects.
	// The full annotation name is supposed to be "<routingClass>.routing.controller.devfile.io/<anything>"
	RoutingAnnotationInfix = ".routing.controller.devfile.io/"

	// DevWorkspaceStorageTypeAtrr defines the strategy used for provisioning storage for the workspace.
	// If empty, the common PVC strategy is used.
	// Supported options:
	// - "common": Create one PVC per namespace, and store data for all workspaces in that namespace in that PVC
	// - "async" : Create one PVC per namespace, and create a remote server that syncs data from workspaces to the PVC.
	//             All volumeMounts used for devworkspaces are emptyDir
	DevWorkspaceStorageTypeAtrr = "controller.devfile.io/storage-type"

	// WorkspaceEndpointNameAnnotation is the annotation key for storing an endpoint's name from the devfile representation
	DevWorkspaceEndpointNameAnnotation = "controller.devfile.io/endpoint_name"

	// DevWorkspaceDiscoverableServiceAnnotation marks a service in a devworkspace as created for a discoverable endpoint,
	// as opposed to a service created to support the devworkspace itself.
	DevWorkspaceDiscoverableServiceAnnotation = "controller.devfile.io/discoverable-service"

	// PullSecretLabel marks the intention that secret should be used as pull secret for devworkspaces withing namespace
	// Only secrets with 'true' value will be mount as pull secret
	// Should be assigned to secrets with type docker config types (kubernetes.io/dockercfg and kubernetes.io/dockerconfigjson)
	DevWorkspacePullSecretLabel = "controller.devfile.io/devworkspace_pullsecret"

	// NamespacedConfigLabelKey is a label applied to configmaps to mark them as a configuration for all DevWorkspaces in
	// the current namespace.
	NamespacedConfigLabelKey = "controller.devfile.io/namespaced-config"
)
