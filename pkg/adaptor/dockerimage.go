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
	"github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	"github.com/che-incubator/che-workspace-operator/pkg/common"
	"github.com/che-incubator/che-workspace-operator/pkg/config"
	corev1 "k8s.io/api/core/v1"
	"strings"
)

func AdaptDockerimageComponents(workspaceId string, devfileComponents []v1alpha1.ComponentSpec, commands []v1alpha1.CommandSpec) ([]v1alpha1.ComponentDescription, error) {
	var components []v1alpha1.ComponentDescription
	for _, devfileComponent := range devfileComponents {
		if devfileComponent.Type != v1alpha1.Dockerimage {
			return nil, fmt.Errorf("cannot adapt non-dockerfile type component %s in docker adaptor", devfileComponent.Alias)
		}
		component, err := adaptDockerimageComponent(workspaceId, devfileComponent, commands)
		if err != nil {
			return nil, err
		}

		components = append(components, component)
	}

	return components, nil
}

func adaptDockerimageComponent(workspaceId string, devfileComponent v1alpha1.ComponentSpec, commands []v1alpha1.CommandSpec) (v1alpha1.ComponentDescription, error) {
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
		Name: devfileComponent.Alias,
		PodAdditions: v1alpha1.PodAdditions{
			Containers: []corev1.Container{container},
		},
		ComponentMetadata: componentMetadata,
	}
	return component, nil
}

func getContainerFromDevfile(workspaceId string, devfileComponent v1alpha1.ComponentSpec) (corev1.Container, v1alpha1.ContainerDescription, error) {
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
		Value: devfileComponent.Alias,
	})

	container := corev1.Container{
		Name:            devfileComponent.Alias,
		Image:           devfileComponent.Image,
		Command:         devfileComponent.Command,
		Args:            devfileComponent.Args,
		Ports:           containerEndpoints,
		Env:             env,
		Resources:       containerResources,
		VolumeMounts:    adaptVolumesMountsFromDevfile(workspaceId, devfileComponent.Volumes),
		ImagePullPolicy: corev1.PullAlways,
	}

	containerDescription := v1alpha1.ContainerDescription{
		Attributes: map[string]string{
			config.RestApisContainerSourceAttribute: config.RestApisRecipeSourceContainerAttribute,
		},
		Ports: endpointInts,
	}
	return container, containerDescription, nil
}

func endpointsToContainerPorts(endpoints []v1alpha1.Endpoint) ([]corev1.ContainerPort, []int) {
	var containerPorts []corev1.ContainerPort
	var containerEndpoints []int

	for _, endpoint := range endpoints {
		containerPorts = append(containerPorts, corev1.ContainerPort{
			Name:          common.EndpointName(endpoint.Name),
			ContainerPort: int32(endpoint.Port),
			//Protocol:      corev1.Protocol(endpoint.Attributes[v1alpha1.PROTOCOL_ENDPOINT_ATTRIBUTE]),
			Protocol: corev1.ProtocolTCP,
		})
		containerEndpoints = append(containerEndpoints, int(endpoint.Port))
	}

	return containerPorts, containerEndpoints
}

func adaptVolumesMountsFromDevfile(workspaceId string, devfileVolumes []v1alpha1.Volume) []corev1.VolumeMount {
	var volumeMounts []corev1.VolumeMount
	volumeName := config.ControllerCfg.GetWorkspacePVCName()

	for _, devfileVolume := range devfileVolumes {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			SubPath:   fmt.Sprintf("%s/%s/", workspaceId, devfileVolume.Name),
			MountPath: devfileVolume.ContainerPath,
		})
	}

	return volumeMounts
}

func GetDockerfileComponentCommands(component v1alpha1.ComponentSpec, commands []v1alpha1.CommandSpec) []v1alpha1.CheWorkspaceCommand {
	var componentCommands []v1alpha1.CheWorkspaceCommand
	for _, command := range commands {
		for _, action := range command.Actions {
			if action.Component == component.Alias {
				attributes := map[string]string{
					config.CommandWorkingDirectoryAttribute:       action.Workdir, // TODO: Env var substitution?
					config.CommandActionReferenceAttribute:        action.Reference,
					config.CommandActionReferenceContentAttribute: action.ReferenceContent,
					config.CommandMachineNameAttribute:            component.Alias,
					config.ComponentAliasCommandAttribute:         action.Component,
				}

				componentCommands = append(componentCommands, v1alpha1.CheWorkspaceCommand{
					Name:        command.Name,
					Type:        action.Type,
					CommandLine: action.Command,
					Attributes:  attributes,
				})
			}
		}
	}
	return componentCommands
}
