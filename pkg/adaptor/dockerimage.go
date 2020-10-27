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

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha1"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/config"
	corev1 "k8s.io/api/core/v1"
)

func AdaptDockerimageComponents(workspaceId string, containerComponents []devworkspace.Component, commands []devworkspace.Command) ([]v1alpha1.ComponentDescription, error) {
	var components []v1alpha1.ComponentDescription
	for _, containerComponent := range containerComponents {
		component, err := adaptDockerimageComponent(workspaceId, *containerComponent.Container, commands)
		if err != nil {
			return nil, err
		}

		components = append(components, component)
	}

	return components, nil
}

func adaptDockerimageComponent(workspaceId string, devfileComponent devworkspace.ContainerComponent, commands []devworkspace.Command) (v1alpha1.ComponentDescription, error) {
	container, containerDescription, err := getContainerFromDevfile(workspaceId, devfileComponent)
	if err != nil {
		return v1alpha1.ComponentDescription{}, nil
	}
	if devfileComponent.MountSources {
		container.VolumeMounts = append(container.VolumeMounts, GetProjectSourcesVolumeMount(workspaceId))
	}

	componentMetadata := v1alpha1.ComponentMetadata{
		Containers: map[string]v1alpha1.ContainerDescription{
			container.Name: containerDescription,
		},
		ContributedRuntimeCommands: GetDockerfileComponentCommands(devfileComponent, commands),
		Endpoints:                  devfileComponent.Endpoints,
	}

	component := v1alpha1.ComponentDescription{
		Name: devfileComponent.Name,
		PodAdditions: v1alpha1.PodAdditions{
			Containers: []corev1.Container{container},
		},
		ComponentMetadata: componentMetadata,
	}
	return component, nil
}

func getContainerFromDevfile(workspaceId string, devfileComponent devworkspace.ContainerComponent) (corev1.Container, v1alpha1.ContainerDescription, error) {
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
		Value: devfileComponent.Name,
	})

	container := corev1.Container{
		Name:            devfileComponent.Name,
		Image:           devfileComponent.Image,
		Command:         devfileComponent.Command,
		Args:            devfileComponent.Args,
		Ports:           containerEndpoints,
		Env:             env,
		Resources:       containerResources,
		VolumeMounts:    adaptVolumesMountsFromDevfile(workspaceId, devfileComponent.VolumeMounts),
		ImagePullPolicy: corev1.PullPolicy(config.ControllerCfg.GetSidecarPullPolicy()),
	}

	containerDescription := v1alpha1.ContainerDescription{
		Attributes: map[string]string{
			config.RestApisContainerSourceAttribute: config.RestApisRecipeSourceContainerAttribute,
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

func GetDockerfileComponentCommands(component devworkspace.ContainerComponent, commands []devworkspace.Command) []v1alpha1.CheWorkspaceCommand {
	var componentCommands []v1alpha1.CheWorkspaceCommand
	for _, command := range commands {
		command.Visit(devworkspace.CommandVisitor{
			Exec: func(exec *devworkspace.ExecCommand) error {
				if exec.Component == component.Name {
					attributes := map[string]string{
						config.CommandWorkingDirectoryAttribute: exec.WorkingDir, // TODO: Env var substitution?
						config.CommandMachineNameAttribute:      component.Name,
						config.ComponentAliasCommandAttribute:   component.Name,
					}

					componentCommands = append(componentCommands, v1alpha1.CheWorkspaceCommand{
						Name:        exec.Id,
						Type:        "exec",
						CommandLine: exec.CommandLine,
						Attributes:  attributes,
					})
				}
				return nil
			},
		})
	}
	return componentCommands
}
