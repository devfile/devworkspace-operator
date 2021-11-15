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

package flatten

import (
	"fmt"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

func resolveWorkspaceEnvVar(flattenedDW *dw.DevWorkspaceTemplateSpec) error {
	workspaceEnv, err := collectWorkspaceEnv(flattenedDW)
	if err != nil {
		return err
	}

	for idx, component := range flattenedDW.Components {
		if component.Container != nil {
			flattenedDW.Components[idx].Container.Env = append(component.Container.Env, workspaceEnv...)
		}
	}

	return nil
}

func collectWorkspaceEnv(flattenedDW *dw.DevWorkspaceTemplateSpec) ([]dw.EnvVar, error) {
	// Use a map to store all workspace env vars to avoid duplicates
	workspaceEnvMap := map[string]string{}

	// Bookkeeping map so that we can format error messages in case of conflict
	envVarToComponent := map[string]string{}

	for _, component := range flattenedDW.Components {
		if !component.Attributes.Exists(constants.WorkspaceEnvAttribute) {
			continue
		}

		var componentEnv []dw.EnvVar
		err := component.Attributes.GetInto(constants.WorkspaceEnvAttribute, &componentEnv)
		if err != nil {
			return nil, fmt.Errorf("failed to read attribute %s on component %s: %w", constants.WorkspaceEnvAttribute, getSourceForComponent(component), err)
		}

		for _, envVar := range componentEnv {
			if existingVal, exists := workspaceEnvMap[envVar.Name]; exists && existingVal != envVar.Value {
				return nil, fmt.Errorf("conflicting definition of environment variable %s in components '%s' and '%s'",
					envVar.Name, envVarToComponent[envVar.Name], getSourceForComponent(component))
			}
			workspaceEnvMap[envVar.Name] = envVar.Value
			envVarToComponent[envVar.Name] = getSourceForComponent(component)
		}
	}

	var workspaceEnv []dw.EnvVar
	for name, value := range workspaceEnvMap {
		workspaceEnv = append(workspaceEnv, dw.EnvVar{Name: name, Value: value})
	}
	return workspaceEnv, nil
}
