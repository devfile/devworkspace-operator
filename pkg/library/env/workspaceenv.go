//
// Copyright (c) 2019-2024 Red Hat, Inc.
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

package env

import (
	"fmt"
	"os"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	devfileConstants "github.com/devfile/devworkspace-operator/pkg/library/constants"
	corev1 "k8s.io/api/core/v1"

	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

// AddCommonEnvironmentVariables adds environment variables to each container in podAdditions. Environment variables added include common
// info environment variables and environment variables defined by a workspaceEnv attribute in the devfile itself
func AddCommonEnvironmentVariables(podAdditions *v1alpha1.PodAdditions, clusterDW *common.DevWorkspaceWithConfig, flattenedDW *dw.DevWorkspaceTemplateSpec) error {
	commonEnv := commonEnvironmentVariables(clusterDW)
	workspaceEnv, err := collectWorkspaceEnv(flattenedDW)
	if err != nil {
		return err
	}
	for idx := range podAdditions.Containers {
		podAdditions.Containers[idx].Env = append(podAdditions.Containers[idx].Env, commonEnv...)
		podAdditions.Containers[idx].Env = append(podAdditions.Containers[idx].Env, workspaceEnv...)
	}
	for idx := range podAdditions.InitContainers {
		podAdditions.InitContainers[idx].Env = append(podAdditions.InitContainers[idx].Env, commonEnv...)
		podAdditions.InitContainers[idx].Env = append(podAdditions.InitContainers[idx].Env, workspaceEnv...)
	}
	return nil
}

func GetEnvironmentVariablesForProjectClone(workspace *common.DevWorkspaceWithConfig) []corev1.EnvVar {
	var cloneEnv []corev1.EnvVar
	cloneEnv = append(cloneEnv, workspace.Config.Workspace.ProjectCloneConfig.Env...)
	cloneEnv = append(cloneEnv, commonEnvironmentVariables(workspace)...)
	cloneEnv = append(cloneEnv, corev1.EnvVar{
		Name:  devfileConstants.ProjectsRootEnvVar,
		Value: constants.DefaultProjectsSourcesRoot,
	})
	return cloneEnv
}

func commonEnvironmentVariables(workspaceWithConfig *common.DevWorkspaceWithConfig) []corev1.EnvVar {
	envvars := []corev1.EnvVar{
		{
			Name:  constants.DevWorkspaceNamespace,
			Value: workspaceWithConfig.Namespace,
		},
		{
			Name:  constants.DevWorkspaceName,
			Value: workspaceWithConfig.Name,
		},
		{
			Name:  constants.DevWorkspaceId,
			Value: workspaceWithConfig.Status.DevWorkspaceId,
		},
		{
			Name:  constants.DevWorkspaceCreator,
			Value: workspaceWithConfig.Labels[constants.DevWorkspaceCreatorLabel],
		},
		{
			Name:  constants.DevWorkspaceIdleTimeout,
			Value: workspaceWithConfig.Config.Workspace.IdleTimeout,
		},
	}

	envvars = append(envvars, GetProxyEnvVars(workspaceWithConfig.Config.Routing.ProxyConfig)...)

	return envvars
}

func GetProxyEnvVars(proxyConfig *v1alpha1.Proxy) []corev1.EnvVar {
	if proxyConfig == nil {
		return nil
	}

	// If httpProxy and httpsProxy are both unset, ignore any value for noProxy``
	if proxyConfig.HttpProxy == nil && proxyConfig.HttpsProxy == nil {
		return nil
	}

	// Proxy env vars are defined by consensus rather than standard; most tools use the lower-snake-case version
	// but some may only look at the upper-snake-case version, so we add both.
	var env []corev1.EnvVar
	if proxyConfig.HttpProxy != nil {
		env = append(env, corev1.EnvVar{Name: "http_proxy", Value: *proxyConfig.HttpProxy})
		env = append(env, corev1.EnvVar{Name: "HTTP_PROXY", Value: *proxyConfig.HttpProxy})
	}
	if proxyConfig.HttpsProxy != nil {
		env = append(env, corev1.EnvVar{Name: "https_proxy", Value: *proxyConfig.HttpsProxy})
		env = append(env, corev1.EnvVar{Name: "HTTPS_PROXY", Value: *proxyConfig.HttpsProxy})
	}
	if proxyConfig.NoProxy != nil {
		// Adding 'KUBERNETES_SERVICE_HOST' env var to the 'no_proxy / NO_PROXY' list. Hot Fix for https://issues.redhat.com/browse/CRW-2820
		kubernetesServiceHost := os.Getenv("KUBERNETES_SERVICE_HOST")
		env = append(env, corev1.EnvVar{Name: "no_proxy", Value: *proxyConfig.NoProxy + "," + kubernetesServiceHost})
		env = append(env, corev1.EnvVar{Name: "NO_PROXY", Value: *proxyConfig.NoProxy + "," + kubernetesServiceHost})
	}

	return env
}

func collectWorkspaceEnv(flattenedDW *dw.DevWorkspaceTemplateSpec) ([]corev1.EnvVar, error) {
	// Use a map to store all workspace env vars to avoid duplicates
	workspaceEnvMap := map[string]string{}

	// Bookkeeping map so that we can format error messages in case of conflict
	envVarToComponent := map[string]string{}

	if flattenedDW.Attributes.Exists(constants.WorkspaceEnvAttribute) {
		var dwEnv []dw.EnvVar
		err := flattenedDW.Attributes.GetInto(constants.WorkspaceEnvAttribute, &dwEnv)
		if err != nil {
			return nil, fmt.Errorf("failed to read attribute %s in DevWorkspace attributes: %w", constants.WorkspaceEnvAttribute, err)
		}
		for _, envVar := range dwEnv {
			if existingVal, exists := workspaceEnvMap[envVar.Name]; exists && existingVal != envVar.Value {
				return nil, fmt.Errorf("conflicting definition of environment variable %s in DevWorkspace attributes",
					envVar.Name)
			}
			workspaceEnvMap[envVar.Name] = envVar.Value
			envVarToComponent[envVar.Name] = "DevWorkspace attributes"
		}
	}

	for _, component := range flattenedDW.Components {
		if !component.Attributes.Exists(constants.WorkspaceEnvAttribute) {
			continue
		}

		var componentEnv []dw.EnvVar
		err := component.Attributes.GetInto(constants.WorkspaceEnvAttribute, &componentEnv)
		if err != nil {
			return nil, fmt.Errorf("failed to read attribute %s on %s: %w", constants.WorkspaceEnvAttribute, getSourceForComponent(component), err)
		}

		for _, envVar := range componentEnv {
			if existingVal, exists := workspaceEnvMap[envVar.Name]; exists && existingVal != envVar.Value {
				return nil, fmt.Errorf("conflicting definition of environment variable %s in %s and %s",
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
// component name otherwise. Returned name is prefixed with "component " -- e.g. "component myComponent"
//
// The purpose of this function is mainly to enable providing better messages to end-users, as a component name may
// not match the name of the plugin in the original DevWorkspace.
func getSourceForComponent(component dw.Component) string {
	if component.Attributes.Exists(constants.PluginSourceAttribute) {
		var err error
		componentName := component.Attributes.GetString(constants.PluginSourceAttribute, &err)
		if err == nil {
			return fmt.Sprintf("component %s", componentName)
		}
	}
	return fmt.Sprintf("component %s", component.Name)
}
