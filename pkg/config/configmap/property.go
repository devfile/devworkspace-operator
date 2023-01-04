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

package configmap

const (
	// image pull policy that is applied to every container within workspace
	sidecarPullPolicy        = "devworkspace.sidecar.image_pull_policy"
	defaultSidecarPullPolicy = "Always"

	// workspacePVCName config property handles the PVC name that should be created and used for all workspaces within one kubernetes namespace
	workspacePVCName        = "devworkspace.pvc.name"
	defaultWorkspacePVCName = "claim-devworkspace"

	workspacePVCStorageClassName = "devworkspace.pvc.storage_class.name"

	// routingClass defines the default routing class that should be used if user does not specify it explicitly
	routingClass        = "devworkspace.default_routing_class"
	defaultRoutingClass = "basic"

	// routingSuffix is the base domain for routes/ingresses created on the cluster. All
	// routes/ingresses will be created with URL http(s)://<unique-to-workspace-part>.<routingSuffix>
	// is supposed to be used by embedded routing solvers only
	routingSuffix = "devworkspace.routing.cluster_host_suffix"

	experimentalFeaturesEnabled        = "devworkspace.experimental_features_enabled"
	defaultExperimentalFeaturesEnabled = "false"

	devworkspaceIdleTimeout        = "devworkspace.idle_timeout"
	defaultDevWorkspaceIdleTimeout = "15m"
)
