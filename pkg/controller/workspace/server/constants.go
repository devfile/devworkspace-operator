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

package server

const (
	MEMORY_LIMIT_ATTRIBUTE   = "memoryLimitBytes"
	MEMORY_REQUEST_ATTRIBUTE = "memoryRequestBytes"

	// Attribute of Runtime Machine to mark source of the container.
	CONTAINER_SOURCE_ATTRIBUTE = "source"

	//Attribute of {@link Machine} that indicates by which plugin this machines is provisioned
	//
	// It contains plugin id, like "plugin": "eclipse/che-theia/master"
	PLUGIN_MACHINE_ATTRIBUTE = "plugin"

	// Mark containers applied to workspace with help recipe definition.
	RECIPE_CONTAINER_SOURCE = "recipe"

	// Mark containers created workspace api like tooling for user development.
	TOOL_CONTAINER_SOURCE = "tool"

	// The projects volume has a standard name used in a couple of locations.
	PROJECTS_VOLUME_NAME = "projects"

	// Attribute of {@link Server} that specifies routing of which port created the server
	SERVER_PORT_ATTRIBUTE = "port"

	// Command attribute which indicates the working directory where the given command must be run
	COMMAND_WORKING_DIRECTORY_ATTRIBUTE = "workingDir"

	// Command attribute which indicates in which machine command must be run. It is optional,
	// IDE should asks user to choose machine if null.
	COMMAND_MACHINE_NAME_ATTRIBUTE = "machineName"

	// Command attribute which indicates in which plugin command must be run. If specified
	// plugin has multiple containers then first containers should be used. Attribute value has the
	// following format: `{PLUGIN_PUBLISHER}/{PLUGIN_NAME}/{PLUGIN_VERSION}`. For example:
	// eclipse/sample-plugin/0.0.1
	COMMAND_PLUGIN_ATTRIBUTE = "plugin"

	// An attribute of the command to store the original path to the file that contains the editor
	// specific configuration.
	COMMAND_ACTION_REFERENCE_ATTRIBUTE = "actionReference"

	// The contents of editor-specific content
	COMMAND_ACTION_REFERENCE_CONTENT_ATTRIBUTE = "actionReferenceContent"

	// Workspace command attributes that indicates with which component it is associated. */
	COMPONENT_ALIAS_COMMAND_ATTRIBUTE = "componentAlias"

	DEPLOYMENT_NAME_LABEL = "deployment"
)
