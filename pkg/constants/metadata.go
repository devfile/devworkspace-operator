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
	// DevWorkspaceIDLabel is label key to store workspace identifier
	DevWorkspaceIDLabel = "controller.devfile.io/devworkspace_id"

	// DevWorkspaceCreatorLabel is the label key for storing the UID of the user who created the workspace
	DevWorkspaceCreatorLabel = "controller.devfile.io/creator"

	// DevWorkspaceNameLabel is label key to store workspace name
	DevWorkspaceNameLabel = "controller.devfile.io/devworkspace_name"

	// DevWorkspaceRestrictedAccessAnnotation marks the intention that devworkspace access is restricted to only the creator; setting this
	// annotation will cause devworkspace start to fail if webhooks are disabled.
	// Operator also propagates it to the devworkspace-related objects to perform authorization.
	DevWorkspaceRestrictedAccessAnnotation = "controller.devfile.io/restricted-access"

	// DevWorkspaceStopReasonAnnotation marks the reason why the devworkspace was stopped; when a devworkspace is restarted
	// this annotation will be cleared
	DevWorkspaceStopReasonAnnotation = "controller.devfile.io/stopped-by"

	// WebhookRestartedAtAnnotation holds the the time (unixnano) of when the webhook server was forced to restart by controller
	WebhookRestartedAtAnnotation = "controller.devfile.io/restarted-at"

	// RoutingAnnotationInfix is the infix of the annotations of DevWorkspace that are passed down as annotation to the DevWorkspaceRouting objects.
	// The full annotation name is supposed to be "<routingClass>.routing.controller.devfile.io/<anything>"
	RoutingAnnotationInfix = ".routing.controller.devfile.io/"

	// DevWorkspaceStorageTypeLabel defines the strategy used for provisioning storage for the workspace.
	// If empty, the common PVC strategy is used.
	// Supported options:
	// - "common": Create one PVC per namespace, and store data for all workspaces in that namespace in that PVC
	// - "async" : Create one PVC per namespace, and create a remote server that syncs data from workspaces to the PVC.
	//             All volumeMounts used for devworkspaces are emptyDir
	DevWorkspaceStorageTypeLabel = "controller.devfile.io/storage-type"

	// WorkspaceEndpointNameAnnotation is the annotation key for storing an endpoint's name from the devfile representation
	DevWorkspaceEndpointNameAnnotation = "controller.devfile.io/endpoint_name"

	// DevWorkspaceDiscoverableServiceAnnotation marks a service in a devworkspace as created for a discoverable endpoint,
	// as opposed to a service created to support the devworkspace itself.
	DevWorkspaceDiscoverableServiceAnnotation = "controller.devfile.io/discoverable-service"

	// PullSecretLabel marks the intention that secret should be used as pull secret for devworkspaces withing namespace
	// Only secrets with 'true' value will be mount as pull secret
	// Should be assigned to secrets with type docker config types (kubernetes.io/dockercfg and kubernetes.io/dockerconfigjson)
	DevWorkspacePullSecretLabel = "controller.devfile.io/devworkspace_pullsecret"
)
