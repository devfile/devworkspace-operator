//
// Copyright (c) 2019-2022 Red Hat, Inc.
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

// Constants that are used in attributes on DevWorkspace elements (components, endpoints, etc.)
const (
	// DevWorkspaceStorageTypeAttribute defines the strategy used for provisioning storage for the workspace.
	// If empty, the common PVC strategy is used.
	// Supported options:
	// - "common":        Create one PVC per namespace, and store data for all workspaces in that namespace in that PVC
	// - "async" :        Create one PVC per namespace, and create a remote server that syncs data from workspaces to the PVC.
	//                    All volumeMounts used for devworkspaces are emptyDir
	// - "per-workspace": Create one PVC per workspace, delete that PVC when the workspace is deleted.
	// - "ephemeral":     Use emptyDir volumes for all volumes in the DevWorkspace. All data is lost when the workspace is
	//                    stopped.
	DevWorkspaceStorageTypeAttribute = "controller.devfile.io/storage-type"

	// RuntimeClassNameAttribute is an attribute added to a DevWorkspace to specify a runtimeClassName for container
	// components in the DevWorkspace (pod.spec.runtimeClassName). If empty, no runtimeClassName is added.
	RuntimeClassNameAttribute = "controller.devfile.io/runtime-class"

	// WorkspaceEnvAttribute is an attribute that specifies a set of environment variables provided by a component
	// that should be added to all workspace containers. The structure of the attribute value should be a list of
	// Devfile 2.0 EnvVar, e.g.
	//
	//   attributes:
	//     workspaceEnv:
	//       - name: ENV_1
	//         value: VAL_1
	//       - name: ENV_2
	//         value: VAL_2
	WorkspaceEnvAttribute = "workspaceEnv"

	// WorkspaceSCCAttribute defines additional SCCs that should be added to the DevWorkspace. The user adding
	// this attribute to a workspace must have the RBAC permissions to "use" the SCC with the given name. For example,
	// to add the 'anyuid' SCC to the workspace Pod, the DevWorkspace should contain
	//
	//     spec:
	//       template:
	//         attributes:
	//           controller.devfile.io/scc: "anyuid"
	//
	// Creating a workspace with this attribute, or updating an existing workspace to include this attribute will fail
	// if the user making the request does not have the "use" permission for the "anyuid" SCC.
	// Only supported on OpenShift.
	WorkspaceSCCAttribute = "controller.devfile.io/scc"

	// ProjectCloneAttribute configures how the DevWorkspace will treat project cloning. By default, an init container
	// will be added to the workspace deployment to clone projects to the workspace before it starts. This attribute
	// must be applied to top-level attributes field in the DevWorkspace.
	// Supported options:
	// - "disable" - Disable automatic project cloning. No init container will be added to the workspace and projects
	//               will not be cloned into the workspace on start.
	ProjectCloneAttribute = "controller.devfile.io/project-clone"

	// PluginSourceAttribute is an attribute added to components, commands, and projects in a flattened
	// DevWorkspace representation to signify where the respective component came from (i.e. which plugin
	// or parent imported it)
	PluginSourceAttribute = "controller.devfile.io/imported-by"

	// EndpointURLAttribute is an attribute added to endpoints to denote the endpoint on the cluster that
	// was created to route to this endpoint
	EndpointURLAttribute = "controller.devfile.io/endpoint-url"
)
