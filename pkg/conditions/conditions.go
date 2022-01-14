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

package conditions

import dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"

const (
	Started              dw.DevWorkspaceConditionType = "Started"
	PullSecretsReady     dw.DevWorkspaceConditionType = "PullSecretsReady"
	DevWorkspaceResolved dw.DevWorkspaceConditionType = "DevWorkspaceResolved"
	StorageReady         dw.DevWorkspaceConditionType = "StorageReady"
	DeploymentReady      dw.DevWorkspaceConditionType = "DeploymentReady"
	DevWorkspaceWarning  dw.DevWorkspaceConditionType = "DevWorkspaceWarning"
)

func GetConditionByType(conditions []dw.DevWorkspaceCondition, t dw.DevWorkspaceConditionType) *dw.DevWorkspaceCondition {
	for _, condition := range conditions {
		if condition.Type == t {
			return &condition
		}
	}
	return nil
}
