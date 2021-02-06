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

// package shim contains functions for generating metadata needed by Che-Theia for correct
// representation of workspaces. These functions serve to enable compatibility between devfile 2.0
// workspaces and Che-Theia until Theia gets devfile 2.0 support.
package shim

import (
	"fmt"

	"github.com/devfile/api/pkg/attributes"
	"github.com/devfile/devworkspace-operator/pkg/config"

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func FillDefaultEnvVars(podAdditions *v1alpha1.PodAdditions, workspace devworkspace.DevWorkspace) {
	for idx, mainContainer := range podAdditions.Containers {
		podAdditions.Containers[idx].Env = append(mainContainer.Env, defaultEnvVars(mainContainer.Name, workspace)...)
	}

	for idx, mainContainer := range podAdditions.InitContainers {
		podAdditions.InitContainers[idx].Env = append(mainContainer.Env, defaultEnvVars(mainContainer.Name, workspace)...)
	}
}

func defaultEnvVars(containerName string, workspace devworkspace.DevWorkspace) []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "CHE_MACHINE_NAME",
			Value: containerName,
		},
		{
			Name: "CHE_MACHINE_TOKEN",
		},
		{
			Name:  "CHE_PROJECTS_ROOT",
			Value: config.DefaultProjectsSourcesRoot,
		},
		{
			Name:  "CHE_API",
			Value: config.DefaultApiEndpoint,
		},
		{
			Name:  "CHE_API_INTERNAL",
			Value: config.DefaultApiEndpoint,
		},
		{
			Name:  "CHE_API_EXTERNAL",
			Value: config.DefaultApiEndpoint,
		},
		{
			Name:  "CHE_WORKSPACE_NAME",
			Value: workspace.Name,
		},
		{
			Name:  "CHE_WORKSPACE_ID",
			Value: workspace.Status.WorkspaceId,
		},
		{
			Name:  "CHE_AUTH_ENABLED",
			Value: config.AuthEnabled,
		},
		{
			Name:  "CHE_WORKSPACE_NAMESPACE",
			Value: workspace.Namespace,
		},
	}
}

func GetComponentDescriptionsFromPodAdditions(podAdditions *v1alpha1.PodAdditions, workspace devworkspace.DevWorkspaceTemplateSpec) ([]v1alpha1.ComponentDescription, error) {
	var descriptions []v1alpha1.ComponentDescription

	for _, mainContainer := range podAdditions.Containers {
		component, err := GetComponentByName(mainContainer.Name, workspace)
		if err != nil {
			return nil, err
		}
		descriptions = append(descriptions, v1alpha1.ComponentDescription{
			Name: component.Name,
			PodAdditions: v1alpha1.PodAdditions{
				Containers: []corev1.Container{mainContainer},
			},
			ComponentMetadata: GetComponentMetadata(component.Name, component.Container, workspace.Commands, component.Attributes),
		})
	}

	for _, initContainer := range podAdditions.InitContainers {
		component, err := GetComponentByName(initContainer.Name, workspace)
		if err != nil {
			return nil, err
		}
		descriptions = append(descriptions, v1alpha1.ComponentDescription{
			Name: component.Name,
			PodAdditions: v1alpha1.PodAdditions{
				InitContainers: []corev1.Container{initContainer},
			},
			ComponentMetadata: GetComponentMetadata(component.Name, component.Container, workspace.Commands, component.Attributes),
		})
	}

	// TODO: It's unclear how ComponentDescriptions accomodates volumes, especially with e.g. the common
	// TODO: PVC strategy; no *one* component defines the common PVC, and we don't support multiple components
	// TODO: defining the same volume. To work around this, we create a dummy component to store volumes.
	descriptions = append(descriptions, v1alpha1.ComponentDescription{
		Name: "devfile-storage-volumes",
		PodAdditions: v1alpha1.PodAdditions{
			Volumes: podAdditions.Volumes,
		},
	})

	return descriptions, nil
}

func GetComponentByName(name string, workspace devworkspace.DevWorkspaceTemplateSpec) (*devworkspace.Component, error) {
	for _, component := range workspace.Components {
		if name == component.Key() {
			return &component, nil
		}
	}
	return nil, fmt.Errorf("component with name %s not found", name)
}

func GetComponentMetadata(componentName string, container *devworkspace.ContainerComponent, commands []devworkspace.Command, attr attributes.Attributes) v1alpha1.ComponentMetadata {
	componentMetadata := v1alpha1.ComponentMetadata{
		Containers: map[string]v1alpha1.ContainerDescription{
			componentName: getComponentContainerDescription(container, attr),
		},
		ContributedRuntimeCommands: getDockerfileComponentCommands(componentName, commands),
		Endpoints:                  container.Endpoints,
	}
	return componentMetadata
}

func getComponentContainerDescription(container *devworkspace.ContainerComponent, attributes attributes.Attributes) v1alpha1.ContainerDescription {
	source := config.RestApisRecipeSourceContainerAttribute
	if attributes.Exists("app.kubernetes.io/component") {
		source = config.RestApisRecipeSourceToolAttribute
	}

	var ports []int
	for _, endpoint := range container.Endpoints {
		ports = append(ports, endpoint.TargetPort)
	}

	return v1alpha1.ContainerDescription{
		Attributes: map[string]string{
			config.RestApisContainerSourceAttribute: source,
		},
		Ports: ports,
	}
}

func getDockerfileComponentCommands(componentName string, commands []devworkspace.Command) []v1alpha1.CheWorkspaceCommand {
	var componentCommands []v1alpha1.CheWorkspaceCommand
	for _, command := range commands {
		if command.Exec == nil {
			continue
		}
		if command.Exec.Component == componentName {
			attr := map[string]string{
				config.CommandWorkingDirectoryAttribute: command.Exec.WorkingDir, // TODO: Env var substitution?
				config.CommandMachineNameAttribute:      componentName,
				config.ComponentAliasCommandAttribute:   componentName,
			}

			componentCommands = append(componentCommands, v1alpha1.CheWorkspaceCommand{
				Name:        command.Id,
				Type:        "exec",
				CommandLine: command.Exec.CommandLine,
				Attributes:  attr,
			})
		}
	}
	return componentCommands
}
