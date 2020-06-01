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

const (
	pluginRegistryURL = "plugin.registry.url"

	routingSuffix        = "cluster.routing_suffix"
	defaultRoutingSuffix = ""

	webhooksEnabled        = "che.webhooks.enabled"
	defaultWebhooksEnabled = "true"

	cheAPISidecarImage = "che.workspace.che_api_sidecar.image"
	// by default that functionality is not available since it's not fully supported
	defaultCheAPISidecarImage = ""

	sidecarPullPolicy        = "che.workspace.sidecar.image_pull_policy"
	defaultSidecarPullPolicy = "Always"

	// workspacePVCName config property handles the PVC name that should be created and used for all workspaces within one kubernetes namespace
	workspacePVCName        = "che.workspace.pvc.name"
	defaultWorkspacePVCName = "claim-che-workspace"

	workspacePVCStorageClassName = "che.workspace_pvc.storage_class.name"

	pluginArtifactsBrokerImage        = "che.workspace.plugin_broker.artifacts.image"
	defaultPluginArtifactsBrokerImage = "quay.io/eclipse/che-plugin-artifacts-broker:v3.1.0"

	// routingClass defines the default routing class that should be used if user does not specify it explicitly
	routingClass        = "che.workspace.default_routing_class"
	defaultRoutingClass = "basic"

	workspaceIdleTimeout        = "che.workspace.idle_timeout"
	defaultWorkspaceIdleTimeout = "15m"
)
