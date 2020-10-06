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
	// URL of external plugin registry; will be used when devworkspace uses plugin
	// not included in internal registry or when devfile does not include explicit
	// registry URL.
	pluginRegistryURL = "controller.plugin_registry.url"

	webhooksEnabled        = "controller.webhooks.enabled"
	defaultWebhooksEnabled = "true"

	cheAPISidecarImage = "devworkspace.api_sidecar.image"
	// by default that functionality is not available since it's not fully supported
	defaultCheAPISidecarImage = ""

	// image pull policy that is applied to every container within workspace
	sidecarPullPolicy        = "devworkspace.sidecar.image_pull_policy"
	defaultSidecarPullPolicy = "Always"

	// workspacePVCName config property handles the PVC name that should be created and used for all workspaces within one kubernetes namespace
	workspacePVCName        = "devworkspace.pvc.name"
	defaultWorkspacePVCName = "claim-devworkspace"

	workspacePVCStorageClassName = "devworkspace.pvc.storage_class.name"

	pluginArtifactsBrokerImage        = "controller.plugin_artifacts_broker.image"
	defaultPluginArtifactsBrokerImage = "quay.io/eclipse/che-plugin-artifacts-broker:v3.1.0"

	// routingClass defines the default routing class that should be used if user does not specify it explicitly
	routingClass        = "devworkspace.default_routing_class"
	defaultRoutingClass = "basic"

	// routingSuffix is the default domain for routes/ingresses created on the cluster. All
	// routes/ingresses will be created with URL http(s)://<unique-to-workspace-part>.<routingSuffix>
	routingSuffix        = "devworkspace.routing.cluster_host_suffix"
	defaultRoutingSuffix = ""

	experimentalFeaturesEnabled        = "devworkspace.experimental_features_enabled"
	defaultExperimentalFeaturesEnabled = "false"

	workspaceIdleTimeout        = "devworkspace.idle_timeout"
	defaultWorkspaceIdleTimeout = "15m"

	// Skip Verify for TLS connections
	// It's insecure and should be used only for testing
	tlsInsecureSkipVerify        = "tls.insecure_skip_verify"
	defaultTlsInsecureSkipVerify = "false"
)
