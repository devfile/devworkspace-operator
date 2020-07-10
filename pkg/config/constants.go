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

// Labels which should be used for controller related objects
var ControllerAppLabels = func() map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":    "devworkspace-controller",
		"app.kubernetes.io/part-of": "devworkspace-operator",
	}
}

// Internal constants
const (
	// default URL for accessing Che Rest API Emulator from Workspace containers
	DefaultApiEndpoint = "http://localhost:9999/api/"

	DefaultProjectsSourcesRoot = "/projects"

	DefaultPluginsVolumeName = "plugins"
	PluginsMountPath         = "/plugins"

	AuthEnabled = "false"

	ServiceAccount = "devworkspace"

	SidecarDefaultMemoryLimit = "128M"
	PVCStorageSize            = "1Gi"

	// WorkspaceIDLabel is label key to store workspace identifier
	WorkspaceIDLabel = "controller.devfile.io/workspace_id"

	// WorkspaceEndpointNameAnnotation is the annotation key for storing an endpoint's name from the devfile representation
	WorkspaceEndpointNameAnnotation = "controller.devfile.io/endpoint_name"

	// WorkspaceNameLabel is label key to store workspace name
	WorkspaceNameLabel = "controller.devfile.io/workspace_name"

	// WorkspaceCreatorLabel is the label key for storing the UID of the user who created the workspace
	WorkspaceCreatorLabel = "controller.devfile.io/creator"

	// WorkspaceImmutableAnnotation marks the intention that workspace access is restricted to only the creator; setting this
	// annotation will cause workspace start to fail if webhooks are disabled.
	WorkspaceImmutableAnnotation = "controller.devfile.io/restricted-access"

	// WorkspaceDiscoverableServiceAnnotation marks a service in a workspace as created for a discoverable endpoint,
	// as opposed to a service created to support the workspace itself.
	WorkspaceDiscoverableServiceAnnotation = "controller.devfile.io/discoverable-service"

	// ControllerServiceAccountNameEnvVar stores the name of the serviceaccount used in the controller.
	ControllerServiceAccountNameEnvVar = "CONTROLLER_SERVICE_ACCOUNT_NAME"
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

	// RestAPIsRuntimeVolumePathis the path where workspace information is mounted in che-rest-apis
	RestAPIsRuntimeVolumePath = "/workspace/"

	// RestAPIsRuntimeJSONFilename is the filename for the runtime json annotation
	RestAPIsRuntimeJSONFilename = "runtime.json"

	// RestAPIsDevfileYamlFilename is the filename for the devfile yaml
	RestAPIsDevfileYamlFilename = "devfile.yaml"

	// RestAPIsVolumeName is the name of the configmap volume that stores workspace metadata
	RestAPIsVolumeName = "che-rest-apis-data"
)

// constants for workspace controller
const (
	// The IDE of theia editor in devfile
	TheiaEditorID = "eclipse/che-theia"
)
