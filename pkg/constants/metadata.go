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
