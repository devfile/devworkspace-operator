//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package flatten

import (
	"fmt"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
)

const (
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
		if !component.Attributes.Exists(WorkspaceEnvAttribute) {
			continue
		}

		var componentEnv []dw.EnvVar
		err := component.Attributes.GetInto(WorkspaceEnvAttribute, &componentEnv)
		if err != nil {
			return nil, fmt.Errorf("failed to read attribute %s on component %s: %w", WorkspaceEnvAttribute, getSourceForComponent(component), err)
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
