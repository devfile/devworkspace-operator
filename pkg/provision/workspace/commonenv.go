//
// Copyright (c) 2019-2021 Red Hat, Inc.
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

package workspace

import (
	v1 "k8s.io/api/core/v1"

	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

func CommonEnvironmentVariables(workspaceName, workspaceId, namespace, creator string) []v1.EnvVar {
	return []v1.EnvVar{
		{
			Name:  constants.DevWorkspaceNamespace,
			Value: namespace,
		},
		{
			Name:  constants.DevWorkspaceName,
			Value: workspaceName,
		},
		{
			Name:  constants.DevWorkspaceId,
			Value: workspaceId,
		},
		{
			Name:  constants.DevWorkspaceCreator,
			Value: creator,
		},
		{
			Name:  constants.DevWorkspaceIdleTimeout,
			Value: config.Workspace.IdleTimeout,
		},
	}
}
