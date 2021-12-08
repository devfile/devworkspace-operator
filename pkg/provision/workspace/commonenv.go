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
	"fmt"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	corev1 "k8s.io/api/core/v1"

	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

func CommonEnvironmentVariables(workspaceName, workspaceId, namespace, creator string) []corev1.EnvVar {
	return []corev1.EnvVar{
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

func collectWorkspaceEnv(flattenedDW *dw.DevWorkspaceTemplateSpec) ([]corev1.EnvVar, error) {
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

	var workspaceEnv []corev1.EnvVar
	for name, value := range workspaceEnvMap {
		workspaceEnv = append(workspaceEnv, corev1.EnvVar{Name: name, Value: value})
	}
	return workspaceEnv, nil
}

// getSourceForComponent returns the 'original' name for a component in a flattened DevWorkspace. Given a component, it
// returns the name of the plugin component that imported it if the component came via a plugin, and the actual
// component name otherwise.
//
// The purpose of this function is mainly to enable providing better messages to end-users, as a component name may
// not match the name of the plugin in the original DevWorkspace.
func getSourceForComponent(component dw.Component) string {
	if component.Attributes.Exists(constants.PluginSourceAttribute) {
		var err error
		componentName := component.Attributes.GetString(constants.PluginSourceAttribute, &err)
		if err == nil {
			return componentName
		}
	}
	return component.Name
}
