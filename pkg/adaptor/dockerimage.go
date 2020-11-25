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
	"strings"

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/config"
	corev1 "k8s.io/api/core/v1"
)

func AdaptDockerimageComponents(workspaceId string, containerComponents []devworkspace.Component, commands []devworkspace.Command) ([]v1alpha1.ComponentDescription, error) {
	var components []v1alpha1.ComponentDescription
	for _, containerComponent := range containerComponents {
		component, err := adaptDockerimageComponent(workspaceId, containerComponent, commands)
		if err != nil {
			return nil, err
		}
		components = append(components, *component)
	}
	return components, nil
}

func adaptDockerimageComponent(workspaceId string, devfileComponent devworkspace.Component, commands []devworkspace.Command) (*v1alpha1.ComponentDescription, error) {
	if devfileComponent.Container == nil {
		return nil, fmt.Errorf("trying to adapt devfile v1 dockerimage from non-container component")
	}
	devfileContainer := *devfileComponent.Container
	container, containerDescription, err := getContainerFromDevfile(workspaceId, devfileComponent.Key(), devfileComponent)
	if err != nil {
		return nil, err
	}
	if devfileContainer.MountSources != nil && *devfileContainer.MountSources {
		container.VolumeMounts = append(container.VolumeMounts, GetProjectSourcesVolumeMount(workspaceId))
	}

	componentMetadata := v1alpha1.ComponentMetadata{
		Containers: map[string]v1alpha1.ContainerDescription{
			container.Name: containerDescription,
		},
		ContributedRuntimeCommands: GetDockerfileComponentCommands(devfileComponent.Key(), commands),
		Endpoints:                  devfileContainer.Endpoints,
	}

	component := &v1alpha1.ComponentDescription{
		Name: devfileComponent.Name,
		PodAdditions: v1alpha1.PodAdditions{
			Containers: []corev1.Container{container},
		},
		ComponentMetadata: componentMetadata,
	}
	return component, nil
}

func AdaptInitContainerComponents(workspaceId string, containerComponents []devworkspace.Component) ([]v1alpha1.ComponentDescription, error) {
	var initContainerComponents []v1alpha1.ComponentDescription
	for _, component := range containerComponents {
		if component.Container == nil {
			return nil, fmt.Errorf("trying to adapt devfile v1 dockerimage from non-container component")
		}
		initContainerComponent := *component.Container
		container, _, err := getContainerFromDevfile(workspaceId, component.Key(), component)
		if err != nil {
			return nil, err
		}
		if initContainerComponent.MountSources != nil && *initContainerComponent.MountSources {
			container.VolumeMounts = append(container.VolumeMounts, GetProjectSourcesVolumeMount(workspaceId))
		}

		initContainerComponents = append(initContainerComponents, v1alpha1.ComponentDescription{
			Name: component.Name,
			PodAdditions: v1alpha1.PodAdditions{
				InitContainers: []corev1.Container{container},
			},
		})
	}
	return initContainerComponents, nil
}

func getContainerFromDevfile(workspaceId, componentName string, component devworkspace.Component) (corev1.Container, v1alpha1.ContainerDescription, error) {
	devfileComponent := component.Container
	containerResources, err := adaptResourcesFromString(devfileComponent.MemoryLimit)
	if err != nil {
		return corev1.Container{}, v1alpha1.ContainerDescription{}, err
	}
	containerEndpoints, endpointInts := endpointsToContainerPorts(devfileComponent.Endpoints)

	var env []corev1.EnvVar
	for _, devfileEnvVar := range devfileComponent.Env {
		env = append(env, corev1.EnvVar{
			Name:  devfileEnvVar.Name,
			Value: strings.ReplaceAll(devfileEnvVar.Value, "$(CHE_PROJECTS_ROOT)", config.DefaultProjectsSourcesRoot),
		})
	}
	env = append(env, corev1.EnvVar{
		Name:  "CHE_MACHINE_NAME",
		Value: componentName,
	})

	container := corev1.Container{
		Name:            componentName,
		Image:           devfileComponent.Image,
		Command:         devfileComponent.Command,
		Args:            devfileComponent.Args,
		Ports:           containerEndpoints,
		Env:             env,
		Resources:       containerResources,
		VolumeMounts:    adaptVolumesMountsFromDevfile(workspaceId, devfileComponent.VolumeMounts),
		ImagePullPolicy: corev1.PullPolicy(config.ControllerCfg.GetSidecarPullPolicy()),
	}

	source := config.RestApisRecipeSourceContainerAttribute
	if component.Attributes.Exists("app.kubernetes.io/component") {
		source = config.RestApisRecipeSourceToolAttribute
	}

	containerDescription := v1alpha1.ContainerDescription{
		Attributes: map[string]string{
			config.RestApisContainerSourceAttribute: source,
		},
		Ports: endpointInts,
	}
	return container, containerDescription, nil
}

func endpointsToContainerPorts(endpoints []devworkspace.Endpoint) ([]corev1.ContainerPort, []int) {
	var containerPorts []corev1.ContainerPort
	var containerEndpoints []int

	for _, endpoint := range endpoints {
		containerPorts = append(containerPorts, corev1.ContainerPort{
			Name:          common.EndpointName(endpoint.Name),
			ContainerPort: int32(endpoint.TargetPort),
			//Protocol:      corev1.Protocol(endpoint.Attributes[v1alpha1.PROTOCOL_ENDPOINT_ATTRIBUTE]),
			Protocol: corev1.ProtocolTCP,
		})
		containerEndpoints = append(containerEndpoints, int(endpoint.TargetPort))
	}

	return containerPorts, containerEndpoints
}

func adaptVolumesMountsFromDevfile(workspaceId string, devfileVolumes []devworkspace.VolumeMount) []corev1.VolumeMount {
	var volumeMounts []corev1.VolumeMount
	volumeName := config.ControllerCfg.GetWorkspacePVCName()

	for _, devfileVolume := range devfileVolumes {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			SubPath:   fmt.Sprintf("%s/%s/", workspaceId, devfileVolume.Name),
			MountPath: devfileVolume.Path,
		})
	}

	return volumeMounts
}

func GetDockerfileComponentCommands(componentName string, commands []devworkspace.Command) []v1alpha1.CheWorkspaceCommand {
	var componentCommands []v1alpha1.CheWorkspaceCommand
	for _, command := range commands {
		if command.Exec == nil {
			continue
		}
		if command.Exec.Component == componentName {
			attributes := map[string]string{
				config.CommandWorkingDirectoryAttribute: command.Exec.WorkingDir, // TODO: Env var substitution?
				config.CommandMachineNameAttribute:      componentName,
				config.ComponentAliasCommandAttribute:   componentName,
			}

			componentCommands = append(componentCommands, v1alpha1.CheWorkspaceCommand{
				Name:        command.Id,
				Type:        "exec",
				CommandLine: command.Exec.CommandLine,
				Attributes:  attributes,
			})
		}
	}
	return componentCommands
}
