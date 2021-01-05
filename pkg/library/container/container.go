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

// Package container contains library functions for converting DevWorkspace Container components to Kubernetes
// components
package container

import (
	"fmt"

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/library"
	"github.com/devfile/devworkspace-operator/pkg/library/lifecycle"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	ProjectsRootEnvVar   = "PROJECTS_ROOT"
	ProjectsSourceEnvVar = "PROJECTS_SOURCE"
	ProjecstVolumeName   = "projects"
)

// GetKubeContainersFromDevfile converts container components in a DevWorkspace into Kubernetes containers.
// If a DevWorkspace container is an init container (i.e. is bound to a preStart event), it will be returned as an
// init container.
//
// Note: Requires DevWorkspace to be flattened (i.e. the DevWorkspace contains no Parent or Components of type Plugin)
func GetKubeContainersFromDevfile(workspace devworkspace.DevWorkspaceTemplateSpec) (*v1alpha1.PodAdditions, error) {
	if !library.DevWorkspaceIsFlattened(workspace) {
		return nil, fmt.Errorf("devfile is not flattened")
	}
	podAdditions := &v1alpha1.PodAdditions{}

	initContainers, mainComponents, err := lifecycle.GetInitContainers(workspace.DevWorkspaceTemplateSpecContent)
	if err != nil {
		return nil, err
	}

	for _, component := range mainComponents {
		if component.Container == nil {
			continue
		}
		k8sContainer, err := convertContainerToK8s(component)
		if err != nil {
			return nil, err
		}
		podAdditions.Containers = append(podAdditions.Containers, *k8sContainer)
	}

	for _, container := range initContainers {
		k8sContainer, err := convertContainerToK8s(container)
		if err != nil {
			return nil, err
		}
		podAdditions.InitContainers = append(podAdditions.InitContainers, *k8sContainer)
	}

	fillDefaultEnvVars(podAdditions, workspace)

	return podAdditions, nil
}

func convertContainerToK8s(devfileComponent devworkspace.Component) (*corev1.Container, error) {
	if devfileComponent.Container == nil {
		return nil, fmt.Errorf("cannot get k8s container from non-container component")
	}
	devfileContainer := devfileComponent.Container

	containerResources, err := devfileResourcesToContainerResources(devfileContainer)
	if err != nil {
		return nil, err
	}

	var mountSources bool
	if devfileContainer.MountSources == nil {
		mountSources = true
	} else {
		mountSources = *devfileContainer.MountSources
	}

	container := &corev1.Container{
		Name:            devfileComponent.Name,
		Image:           devfileContainer.Image,
		Command:         devfileContainer.Command,
		Args:            devfileContainer.Args,
		Resources:       *containerResources,
		Ports:           devfileEndpointsToContainerPorts(devfileContainer.Endpoints),
		Env:             devfileEnvToContainerEnv(devfileContainer.Env),
		VolumeMounts:    devfileVolumeMountsToContainerVolumeMounts(devfileContainer.VolumeMounts, mountSources),
		ImagePullPolicy: corev1.PullPolicy(config.ControllerCfg.GetSidecarPullPolicy()),
	}

	return container, nil
}

func devfileEndpointsToContainerPorts(endpoints []devworkspace.Endpoint) []corev1.ContainerPort {
	var containerPorts []corev1.ContainerPort
	for _, endpoint := range endpoints {
		containerPorts = append(containerPorts, corev1.ContainerPort{
			Name:          endpoint.Name,
			ContainerPort: int32(endpoint.TargetPort),
			Protocol:      corev1.ProtocolTCP,
		})
	}
	return containerPorts
}

func devfileResourcesToContainerResources(devfileContainer *devworkspace.ContainerComponent) (*corev1.ResourceRequirements, error) {
	// TODO: Handle memory request and CPU when implemented in devfile API
	memLimit := devfileContainer.MemoryLimit
	if memLimit == "" {
		memLimit = config.SidecarDefaultMemoryLimit
	}
	memLimitQuantity, err := resource.ParseQuantity(memLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to parse memory limit %q: %w", memLimit, err)
	}
	return &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: memLimitQuantity,
		},
	}, nil
}

func devfileVolumeMountsToContainerVolumeMounts(devfileVolumeMounts []devworkspace.VolumeMount, mountSources bool) []corev1.VolumeMount {
	var volumeMounts []corev1.VolumeMount
	for _, vm := range devfileVolumeMounts {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      vm.Name,
			MountPath: vm.Path,
		})
	}
	if mountSources {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      ProjecstVolumeName,
			MountPath: config.DefaultProjectsSourcesRoot,
		})
	}
	return volumeMounts
}

func devfileEnvToContainerEnv(devfileEnvVars []devworkspace.EnvVar) []corev1.EnvVar {
	var env []corev1.EnvVar
	for _, devfileEnv := range devfileEnvVars {
		env = append(env, corev1.EnvVar{
			Name:  devfileEnv.Name,
			Value: devfileEnv.Value,
		})
	}
	return env
}

func fillDefaultEnvVars(podAdditions *v1alpha1.PodAdditions, workspace devworkspace.DevWorkspaceTemplateSpec) {
	var projectsSource string
	if len(workspace.Projects) > 0 {
		// TODO: Unclear from devfile spec how this should work when there are multiple projects
		projectsSource = fmt.Sprintf("%s/%s", config.DefaultProjectsSourcesRoot, workspace.Projects[0].ClonePath)
	} else {
		projectsSource = config.DefaultProjectsSourcesRoot
	}

	// Add devfile reserved env var and legacy env var for Che-Theia
	for idx, container := range podAdditions.Containers {
		podAdditions.Containers[idx].Env = append(container.Env, corev1.EnvVar{
			Name:  ProjectsRootEnvVar,
			Value: config.DefaultProjectsSourcesRoot,
		}, corev1.EnvVar{
			Name:  ProjectsSourceEnvVar,
			Value: projectsSource,
		})
	}
	for idx, container := range podAdditions.InitContainers {
		podAdditions.InitContainers[idx].Env = append(container.Env, corev1.EnvVar{
			Name:  ProjectsRootEnvVar,
			Value: config.DefaultProjectsSourcesRoot,
		}, corev1.EnvVar{
			Name:  ProjectsSourceEnvVar,
			Value: projectsSource,
		})
	}
}
