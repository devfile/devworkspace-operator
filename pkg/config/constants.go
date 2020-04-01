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

package config

import "k8s.io/api/admissionregistration/v1beta1"

// Internal constants
const (
	// default URL for accessing Che Rest API Emulator from Workspace containers
	DefaultApiEndpoint = "http://localhost:9999/api/"

	DefaultProjectsSourcesRoot = "/projects"

	DefaultPluginsVolumeName = "plugins"
	PluginsMountPath         = "/plugins"

	CheOriginalName = "workspace"

	AuthEnabled = "false"

	ServiceAccount = "che-workspace"

	SidecarDefaultMemoryLimit = "128M"
	PVCStorageSize            = "1Gi"

	// RuntimeAdditionalInfo is a key of workspaceStatus.AdditionalInfo where runtime info is stored
	RuntimeAdditionalInfo = "org.eclipse.che.workspace/runtime"

	// RuntimeAdditionalInfo is a key of workspaceStatus.AdditionalInfo info where component statuses info is stored
	ComponentStatusesAdditionalInfo = "org.eclipse.che.workspace/componentstatuses"

	// WorkspaceIDLabel is label key to store workspace identifier
	WorkspaceIDLabel = "che.workspace_id"

	// WorkspaceNameLabel is label key to store workspace identifier
	WorkspaceNameLabel = "che.workspace_name"

	// CheOriginalNameLabel is label key to original name
	CheOriginalNameLabel = "che.original_name"

	WorkspaceCreatorAnnotation = "org.eclipse.che.workspace/creator"
)

// Constants for che-rest-apis
const (
	// Attribute of Runtime Machine to mark source of the container.
	RestApisContainerSourceAttribute = "source"
	RestApisPluginMachineAttribute   = "plugin"

	// Mark containers applied to workspace with help recipe definition.
	RestApisRecipeSourceContainerAttribute = "recipe"

	// Mark containers created workspace api like tooling for user development.
	RestApisRecipeSourceToolAttribute = "tool"

	// Command attribute which indicates the working directory where the given command must be run
	CommandWorkingDirectoryAttribute = "workingDir"

	// Command attribute which indicates in which machine command must be run. It is optional,
	// IDE should asks user to choose machine if null.
	CommandMachineNameAttribute = "machineName"

	// Command attribute which indicates in which plugin command must be run. If specified
	// plugin has multiple containers then first containers should be used. Attribute value has the
	// following format: `{PLUGIN_PUBLISHER}/{PLUGIN_NAME}/{PLUGIN_VERSION}`. For example:
	// eclipse/sample-plugin/0.0.1
	CommandPluginAttribute = "plugin"

	// An attribute of the command to store the original path to the file that contains the editor
	// specific configuration.
	CommandActionReferenceAttribute = "actionReference"

	// The contents of editor-specific content
	CommandActionReferenceContentAttribute = "actionReferenceContent"

	// Workspace command attributes that indicates with which component it is associated. */
	ComponentAliasCommandAttribute = "componentAlias"
)

// constants for webhook
const (
	// The address that the webhook will host on
	WebhookServerHost = "0.0.0.0"

	// The port that the webhook will host on
	WebhookServerPort = 8443

	// The directory where the certificate can be found by the webhook server
	WebhookServerCertDir = "/tmp/k8s-webhook-server/serving-certs"
)

// constants for webhook configuration
const (
	// The name of the workspace admission hook
	MutateWebhookCfgName = "mutate-workspace-admission-hooks"

	// The webhooks path on the server
	MutateWebhookPath = "/mutate-workspaces"

	// Policy on webhook failure
	MutateWebhookFailurePolicy = v1beta1.Fail
)
