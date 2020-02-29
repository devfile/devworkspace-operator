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

package component

import (
	"fmt"
	"github.com/che-incubator/che-workspace-operator/pkg/specutils"
	"strconv"
	"strings"

	workspaceApi "github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	"github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/config"
	"github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/model"
	"github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/server"
	brokerModel "github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

func convertToComponentDescription(plugin brokerModel.ChePlugin, wkspCtx model.WorkspaceContext) (*model.ComponentDescription, error) {
	workspaceAdditions, err := getWorkspaceAdditionsFromPlugin(plugin, wkspCtx)
	if err != nil {
		return nil, err
	}
	services := createServicesFromPlugin(plugin, wkspCtx)
	var externalObjects []runtime.Object
	for _, service := range services {
		externalObjects = append(externalObjects, service)
	}
	endpoints := createEndpointsFromPlugin(plugin)
	commands := createCommandsFromPlugin(plugin, wkspCtx)
	containerDescriptions := createDescriptionsFromPlugin(plugin)

	component := &model.ComponentDescription{
		WorkspaceAdditions: workspaceAdditions,
		ExternalObjects:    externalObjects,
		Status: model.ComponentStatus{
			Endpoints:                  endpoints,
			ContributedRuntimeCommands: commands,
			Containers:                 containerDescriptions,
		},
	}

	return component, nil
}

func getWorkspaceAdditionsFromPlugin(plugin brokerModel.ChePlugin, wkspCtx model.WorkspaceContext) (*model.ComponentWorkspaceAdditions, error) {
	containers, err := convertContainers(plugin.Containers, wkspCtx)
	if err != nil {
		return nil, err
	}
	initContainers, err := convertContainers(plugin.InitContainers, wkspCtx)
	if err != nil {
		return nil, err
	}
	return &model.ComponentWorkspaceAdditions{
		Containers:     containers,
		InitContainers: initContainers,
	}, nil
}

func createServicesFromPlugin(plugin brokerModel.ChePlugin, wkspCtx model.WorkspaceContext) []*corev1.Service {
	var services []*corev1.Service
	for _, container := range plugin.Containers {
		for _, service := range createK8sServicesForContainers(wkspCtx, container.Name, exposedPortsToInts(container.Ports)) {
			services = append(services, &service)
		}
	}
	return services
}

func createEndpointsFromPlugin(plugin brokerModel.ChePlugin) []workspaceApi.Endpoint {
	var endpoints []workspaceApi.Endpoint

	for _, pluginEndpoint := range plugin.Endpoints {
		attributes := map[workspaceApi.EndpointAttribute]string{}
		// Default value of http for protocol, may be overwritten by pluginEndpoint attributes
		attributes[workspaceApi.PROTOCOL_ENDPOINT_ATTRIBUTE] = "http"
		attributes[workspaceApi.PUBLIC_ENDPOINT_ATTRIBUTE] = strconv.FormatBool(pluginEndpoint.Public)
		for key, val := range pluginEndpoint.Attributes {
			attributes[workspaceApi.EndpointAttribute(key)] = val
		}
		endpoints = append(endpoints, workspaceApi.Endpoint{
			Name:       pluginEndpoint.Name,
			Port:       int64(pluginEndpoint.TargetPort),
			Attributes: attributes,
		})
	}

	return endpoints
}

func createCommandsFromPlugin(plugin brokerModel.ChePlugin, wkspCtx model.WorkspaceContext) []model.CheWorkspaceCommand {
	var commands []model.CheWorkspaceCommand

	for _, pluginContainer := range plugin.Containers {
		for _, pluginCommand := range pluginContainer.Commands {
			command := model.CheWorkspaceCommand{
				Name:        pluginCommand.Name,
				CommandLine: strings.Join(pluginCommand.Command, " "),
				Type:        "custom",
				Attributes: map[string]string{
					server.COMMAND_WORKING_DIRECTORY_ATTRIBUTE: interpolate(pluginCommand.WorkingDir, wkspCtx),
					server.COMMAND_MACHINE_NAME_ATTRIBUTE:      pluginContainer.Name,
				},
			}
			commands = append(commands, command)
		}
	}

	return commands
}

func createDescriptionsFromPlugin(plugin brokerModel.ChePlugin) map[string]model.ContainerDescription {
	containerDescriptions := map[string]model.ContainerDescription{}
	for _, container := range plugin.Containers {
		attributes := map[string]string{}
		containerResources, err := convertContainerResources(container)
		if err != nil {
			if value, canBeConverted := containerResources.Limits.Memory().AsInt64(); canBeConverted {
				attributes[server.MEMORY_LIMIT_ATTRIBUTE] = strconv.FormatInt(value, 10)
			}
			if value, canBeConverted := containerResources.Requests.Memory().AsInt64(); canBeConverted {
				attributes[server.MEMORY_REQUEST_ATTRIBUTE] = strconv.FormatInt(value, 10)
			}
		}
		attributes[server.CONTAINER_SOURCE_ATTRIBUTE] = server.TOOL_CONTAINER_SOURCE
		attributes[server.PLUGIN_MACHINE_ATTRIBUTE] = plugin.ID

		containerDescriptions[container.Name] = model.ContainerDescription{
			Attributes: attributes,
			Ports:      exposedPortsToInts(container.Ports),
		}
	}
	return containerDescriptions
}

// convertContainers all containers in a plugin to the corev1 spec
func convertContainers(pluginContainers []brokerModel.Container, wkspCtx model.WorkspaceContext) ([]corev1.Container, error) {
	var containers []corev1.Container
	for _, pluginContainer := range pluginContainers {
		container, err := convertContainer(pluginContainer, wkspCtx)
		if err != nil {
			return nil, err
		}
		containers = append(containers, container)
	}
	return containers, nil
}

// convertContainer converts Container model from plugin broker to corev1
func convertContainer(container brokerModel.Container, wkspCtx model.WorkspaceContext) (corev1.Container, error) {
	var converted corev1.Container

	ports := specutils.BuildContainerPorts(exposedPortsToInts(container.Ports), corev1.ProtocolTCP)
	envVars := convertContainerEnvVars(container)
	envVars = append(envVars, commonEnvironmentVariables(wkspCtx)...)
	volumeMounts := createVolumeMounts(wkspCtx, &container.MountSources, nil, container.Volumes)
	resources, err := convertContainerResources(container)
	if err != nil {
		return converted, fmt.Errorf("could not convert container memory limit for %s: %s", container.Name, err)
	}

	converted = corev1.Container{
		Name:                     container.Name,
		Image:                    container.Image,
		ImagePullPolicy:          corev1.PullPolicy(config.ControllerCfg.GetSidecarPullPolicy()),
		Ports:                    ports,
		Resources:                resources,
		Env:                      envVars,
		VolumeMounts:             volumeMounts,
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
	}

	return converted, nil
}

// convertContainerEnvVars converts memory limit/request model from plugin broker to corev1
func convertContainerResources(container brokerModel.Container) (corev1.ResourceRequirements, error) {
	var resourceReqs corev1.ResourceRequirements
	limitStr := container.MemoryLimit
	if limitStr == "" {
		limitStr = "128Mi"
	}
	limit, err := resource.ParseQuantity(limitStr)
	if err != nil {
		return resourceReqs, err
	}
	resourceReqs = corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: limit,
		},
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: limit,
		},
	}
	return resourceReqs, nil
}

// convertContainerEnvVars converts EnvVar model from plugin broker to corev1
func convertContainerEnvVars(container brokerModel.Container) []corev1.EnvVar {
	var envVars []corev1.EnvVar
	for _, env := range container.Env {
		envVars = append(envVars, corev1.EnvVar{
			Name:  env.Name,
			Value: env.Value,
		})
	}
	envVars = append(envVars, corev1.EnvVar{
		Name:  "CHE_MACHINE_NAME",
		Value: container.Name,
	})
	return envVars
}

func exposedPortsToInts(exposedPorts []brokerModel.ExposedPort) []int {
	ports := []int{}
	for _, exposedPort := range exposedPorts {
		ports = append(ports, exposedPort.ExposedPort)
	}
	return ports
}

func getMetasForComponents(components []workspaceApi.ComponentSpec) ([]brokerModel.PluginMeta, error) {
	defaultRegistry := config.ControllerCfg.GetPluginRegistry()
	ioUtils := utils.New()
	var metas []brokerModel.PluginMeta
	for _, component := range components {
		fqn := getPluginFQN(component)
		meta, err := utils.GetPluginMeta(fqn, defaultRegistry, ioUtils)
		if err != nil {
			return nil, err
		}
		metas = append(metas, *meta)
	}
	utils.ResolveRelativeExtensionPaths(metas, defaultRegistry)
	return metas, nil
}

func getPluginFQN(component workspaceApi.ComponentSpec) brokerModel.PluginFQN {
	var pluginFQN brokerModel.PluginFQN
	registryAndID := strings.Split(*(component.Id), "#")
	if len(registryAndID) == 2 {
		pluginFQN.Registry = registryAndID[0]
		pluginFQN.ID = registryAndID[1]
	} else if len(registryAndID) == 1 {
		pluginFQN.ID = registryAndID[0]
	}
	reference := component.Reference
	if reference != nil && *reference != "" {
		pluginFQN.Reference = *reference
	}
	return pluginFQN
}
