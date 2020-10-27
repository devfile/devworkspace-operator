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

package adaptor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/devfile/devworkspace-operator/pkg/adaptor/plugin_patch"

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha1"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"

	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/config"
	registry "github.com/devfile/devworkspace-operator/pkg/internal_registry"
	metadataBroker "github.com/eclipse/che-plugin-broker/brokers/metadata"
	brokerModel "github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/utils"
	corev1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("plugin")

func AdaptPluginComponents(workspaceId, namespace string, devfileComponents []devworkspace.Component) ([]v1alpha1.ComponentDescription, *corev1.ConfigMap, error) {
	var components []v1alpha1.ComponentDescription

	broker := metadataBroker.NewBroker(true)

	metas, _, err := getMetasForComponents(devfileComponents)
	if err != nil {
		return nil, nil, err
	}
	plugins, err := broker.ProcessPlugins(metas)
	if err != nil {
		return nil, nil, err
	}

	for _, plugin := range plugins {
		if config.ControllerCfg.GetExperimentalFeaturesEnabled() {
			plugin_patch.PublicAccessPatch(&plugin)
			plugin_patch.PatchMachineSelector(&plugin, workspaceId)
			plugin_patch.AddMachineNameEnv(&plugin)
		}

		component, err := adaptChePluginToComponent(workspaceId, plugin)
		if err != nil {
			return nil, nil, err
		}
		// TODO: Alias for plugins seems to be ignored in regular Che
		// Setting component.Name = alias here breaks matching, as container names do not match alias
		//if aliases[plugin.ID] != "" {
		//	component.Name = aliases[plugin.ID]
		//}

		components = append(components, component)
	}

	var artifactsBrokerCM *corev1.ConfigMap
	if isArtifactsBrokerNecessary(metas) {
		artifactsBrokerComponent, configMap, err := getArtifactsBrokerComponent(workspaceId, namespace, devfileComponents)
		if err != nil {
			return nil, nil, err
		}
		components = append(components, *artifactsBrokerComponent)
		artifactsBrokerCM = configMap
	}

	return components, artifactsBrokerCM, nil
}

func adaptChePluginToComponent(workspaceId string, plugin brokerModel.ChePlugin) (v1alpha1.ComponentDescription, error) {
	var containers []corev1.Container
	containerDescriptions := map[string]v1alpha1.ContainerDescription{}
	for _, pluginContainer := range plugin.Containers {
		container, containerDescription, err := convertPluginContainer(workspaceId, plugin.ID, pluginContainer)
		if err != nil {
			return v1alpha1.ComponentDescription{}, err
		}
		containers = append(containers, container)
		containerDescriptions[container.Name] = containerDescription
	}
	var initContainers []corev1.Container
	for _, pluginInitContainer := range plugin.InitContainers {
		container, _, err := convertPluginContainer(workspaceId, plugin.ID, pluginInitContainer)
		if err != nil {
			return v1alpha1.ComponentDescription{}, err
		}
		initContainers = append(initContainers, container)
	}

	componentName := plugin.Name
	if len(plugin.Containers) > 0 {
		componentName = plugin.Containers[0].Name
	}
	component := v1alpha1.ComponentDescription{
		Name: componentName,
		PodAdditions: v1alpha1.PodAdditions{
			Containers:     containers,
			InitContainers: initContainers,
		},
		ComponentMetadata: v1alpha1.ComponentMetadata{
			Containers:                 containerDescriptions,
			ContributedRuntimeCommands: GetPluginComponentCommands(plugin), // TODO: Can regular commands apply to plugins in devfile spec?
			Endpoints:                  createEndpointsFromPlugin(plugin),
		},
	}

	return component, nil
}

func createEndpointsFromPlugin(plugin brokerModel.ChePlugin) []devworkspace.Endpoint {
	var endpoints []devworkspace.Endpoint

	for _, pluginEndpoint := range plugin.Endpoints {
		attributes := map[string]string{}
		// Default value of http for protocol, may be overwritten by pluginEndpoint attributes
		attributes[string(v1alpha1.PROTOCOL_ENDPOINT_ATTRIBUTE)] = "http"
		attributes[string(v1alpha1.PUBLIC_ENDPOINT_ATTRIBUTE)] = strconv.FormatBool(pluginEndpoint.Public)
		for key, val := range pluginEndpoint.Attributes {
			attributes[key] = val
		}
		endpoints = append(endpoints, devworkspace.Endpoint{
			Name:       common.EndpointName(pluginEndpoint.Name),
			TargetPort: pluginEndpoint.TargetPort,
			Attributes: attributes,
		})
	}

	return endpoints
}

func convertPluginContainer(workspaceId, pluginID string, brokerContainer brokerModel.Container) (corev1.Container, v1alpha1.ContainerDescription, error) {
	memorylimit := brokerContainer.MemoryLimit
	if memorylimit == "" {
		memorylimit = config.SidecarDefaultMemoryLimit
	}
	containerResources, err := adaptResourcesFromString(memorylimit)
	if err != nil {
		return corev1.Container{}, v1alpha1.ContainerDescription{}, err
	}

	var env []corev1.EnvVar
	for _, brokerEnv := range brokerContainer.Env {
		env = append(env, corev1.EnvVar{
			Name:  brokerEnv.Name,
			Value: strings.ReplaceAll(brokerEnv.Value, "$(CHE_PROJECTS_ROOT)", config.DefaultProjectsSourcesRoot),
		})
	}

	var containerPorts []corev1.ContainerPort
	var portInts []int
	for _, brokerPort := range brokerContainer.Ports {
		containerPorts = append(containerPorts, corev1.ContainerPort{
			ContainerPort: int32(brokerPort.ExposedPort),
			Protocol:      corev1.ProtocolTCP,
		})
		portInts = append(portInts, brokerPort.ExposedPort)
	}

	container := corev1.Container{
		Name:            brokerContainer.Name,
		Image:           brokerContainer.Image,
		Command:         brokerContainer.Command,
		Args:            brokerContainer.Args,
		Ports:           containerPorts,
		Env:             env,
		Resources:       containerResources,
		VolumeMounts:    adaptVolumeMountsFromBroker(workspaceId, brokerContainer),
		ImagePullPolicy: corev1.PullPolicy(config.ControllerCfg.GetSidecarPullPolicy()),
	}

	containerDescription := v1alpha1.ContainerDescription{
		Attributes: map[string]string{
			config.RestApisContainerSourceAttribute: config.RestApisRecipeSourceToolAttribute,
			config.RestApisPluginMachineAttribute:   pluginID,
		},
		Ports: portInts,
	}

	return container, containerDescription, nil
}

func adaptVolumeMountsFromBroker(workspaceId string, brokerContainer brokerModel.Container) []corev1.VolumeMount {
	var volumeMounts []corev1.VolumeMount
	volumeName := config.ControllerCfg.GetWorkspacePVCName()

	// TODO: Handle ephemeral
	for _, brokerVolume := range brokerContainer.Volumes {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			SubPath:   fmt.Sprintf("%s/%s/", workspaceId, brokerVolume.Name),
			MountPath: brokerVolume.MountPath,
		})
	}
	if brokerContainer.MountSources {
		volumeMounts = append(volumeMounts, GetProjectSourcesVolumeMount(workspaceId))
	}

	return volumeMounts
}

func getMetasForComponents(components []devworkspace.Component) (metas []brokerModel.PluginMeta, aliases map[string]string, err error) {
	defaultRegistry := config.ControllerCfg.GetPluginRegistry()
	ioUtils := utils.New()
	aliases = map[string]string{}
	for _, component := range components {
		fqn := getPluginFQN(*component.Plugin)
		var meta *brokerModel.PluginMeta
		// delegate to the internal registry first, if found there then use that
		isInInternalRegistry := registry.IsInInternalRegistry(fqn.ID)
		if isInInternalRegistry {
			meta, err = registry.InternalRegistryPluginToMetaYAML(fqn.ID)
			log.Info(fmt.Sprintf("Grabbing the meta.yaml for %s from the internal registry", fqn.ID))
		} else {
			meta, err = utils.GetPluginMeta(fqn, defaultRegistry, ioUtils)
		}

		if err != nil {
			return nil, nil, err
		}
		metas = append(metas, *meta)
		aliases[meta.ID] = component.Plugin.Name
	}
	err = utils.ResolveRelativeExtensionPaths(metas, defaultRegistry)
	if err != nil {
		return nil, nil, err
	}
	return metas, aliases, nil
}

func getPluginFQN(plugin devworkspace.PluginComponent) brokerModel.PluginFQN {
	var pluginFQN brokerModel.PluginFQN
	registryAndID := strings.Split(plugin.Id, "#")
	if len(registryAndID) == 2 {
		pluginFQN.Registry = registryAndID[0]
		pluginFQN.ID = registryAndID[1]
	} else if len(registryAndID) == 1 {
		pluginFQN.ID = registryAndID[0]
	}
	if plugin.RegistryUrl != "" {
		pluginFQN.Registry = plugin.RegistryUrl
	}
	pluginFQN.Reference = plugin.Uri
	return pluginFQN
}

func GetPluginComponentCommands(plugin brokerModel.ChePlugin) []v1alpha1.CheWorkspaceCommand {
	var commands []v1alpha1.CheWorkspaceCommand

	for _, pluginContainer := range plugin.Containers {
		for _, pluginCommand := range pluginContainer.Commands {
			command := v1alpha1.CheWorkspaceCommand{
				Name:        pluginCommand.Name,
				CommandLine: strings.Join(pluginCommand.Command, " "),
				Type:        "custom",
				Attributes: map[string]string{
					config.CommandWorkingDirectoryAttribute: pluginCommand.WorkingDir, // TODO: Env Var substitution?
					config.CommandMachineNameAttribute:      pluginContainer.Name,
				},
			}
			commands = append(commands, command)
		}
	}

	return commands
}
