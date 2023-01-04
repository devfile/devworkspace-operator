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

package constants

const (
	// DevWorkspaceNamespace contains env var name which value is the current namespace where DevWorkspace CR
	// and related objects live
	DevWorkspaceNamespace = "DEVWORKSPACE_NAMESPACE"

	// DevWorkspaceId contains env var name which which value is the .status.devworkspaceId of the related
	// DevWorkspace CR. It can be used to list related objects with WorkspaceIDLabel selector
	DevWorkspaceId = "DEVWORKSPACE_ID"

	// DevWorkspaceName contains env var name which value is name of the related DevWorkspace CR.
	// It can be used to list related objects with WorkspaceNameLabel selector
	DevWorkspaceName = "DEVWORKSPACE_NAME"

	// DevWorkspaceCreator contains env var name which value is the uid of the identity who created the related devworkspace
	DevWorkspaceCreator = "DEVWORKSPACE_CREATOR"

	// DevWorkspaceIdleTimeout contains env var name which value is the suggested idle timeout
	DevWorkspaceIdleTimeout = "DEVWORKSPACE_IDLE_TIMEOUT"

	// DevWorkspaceComponentName contains env var name which indicates from which devfile container component
	// the container is created from. Note the flattened devfile is used to evaluate it.
	DevWorkspaceComponentName = "DEVWORKSPACE_COMPONENT_NAME"
)
