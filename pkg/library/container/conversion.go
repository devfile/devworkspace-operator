//
// Copyright (c) 2019-2023 Red Hat, Inc.
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

package container

import (
	"fmt"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/common"

	"github.com/devfile/devworkspace-operator/pkg/constants"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func convertContainerToK8s(devfileComponent dw.Component, securityContext *corev1.SecurityContext, pullPolicy string) (*corev1.Container, error) {
	if devfileComponent.Container == nil {
		return nil, fmt.Errorf("cannot get k8s container from non-container component")
	}
	devfileContainer := devfileComponent.Container

	containerResources, err := devfileResourcesToContainerResources(devfileContainer)
	if err != nil {
		return nil, fmt.Errorf("failed to get resources for container %s: %s", devfileComponent.Name, err)
	}

	container := &corev1.Container{
		Name:            devfileComponent.Name,
		Image:           devfileContainer.Image,
		Command:         devfileContainer.Command,
		Args:            devfileContainer.Args,
		Resources:       *containerResources,
		Ports:           devfileEndpointsToContainerPorts(devfileContainer.Endpoints),
		Env:             devfileEnvToContainerEnv(devfileComponent.Name, devfileContainer.Env),
		VolumeMounts:    devfileVolumeMountsToContainerVolumeMounts(devfileContainer.VolumeMounts),
		ImagePullPolicy: corev1.PullPolicy(pullPolicy),
		SecurityContext: securityContext,
	}

	return container, nil
}

func devfileEndpointsToContainerPorts(endpoints []dw.Endpoint) []corev1.ContainerPort {
	var containerPorts []corev1.ContainerPort
	exposedPorts := map[int]bool{}
	for _, endpoint := range endpoints {
		if exposedPorts[endpoint.TargetPort] {
			continue
		}
		containerPorts = append(containerPorts, corev1.ContainerPort{
			// Use meaningless name for port since endpoint.Name does not match requirements for ContainerPort name
			Name:          common.PortName(endpoint),
			ContainerPort: int32(endpoint.TargetPort),
			Protocol:      corev1.ProtocolTCP,
		})
		exposedPorts[endpoint.TargetPort] = true
	}
	return containerPorts
}

func devfileResourcesToContainerResources(devfileContainer *dw.ContainerComponent) (*corev1.ResourceRequirements, error) {
	limits := corev1.ResourceList{}
	requests := corev1.ResourceList{}

	memLimit := devfileContainer.MemoryLimit
	if memLimit == "" {
		memLimit = constants.SidecarDefaultMemoryLimit
	}
	memLimitQuantity, err := resource.ParseQuantity(memLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to parse memory limit %q: %w", memLimit, err)
	}
	limits[corev1.ResourceMemory] = memLimitQuantity

	memReq := devfileContainer.MemoryRequest
	if memReq == "" {
		memReq = constants.SidecarDefaultMemoryRequest
	}
	memReqQuantity, err := resource.ParseQuantity(memReq)
	if err != nil {
		return nil, fmt.Errorf("failed to parse memory request %q: %w", memReq, err)
	}
	requests[corev1.ResourceMemory] = memReqQuantity

	if memLimitQuantity.Cmp(memReqQuantity) < 0 {
		if devfileContainer.MemoryRequest != "" {
			return nil, fmt.Errorf("container resources are invalid: memory limit (%s) is less than request (%s)", memLimit, devfileContainer.MemoryRequest)
		} else {
			// No value was supplied; the issue is that the default value is greater than supplied limit. To resolve this, reuse limit as request
			requests[corev1.ResourceMemory] = memLimitQuantity
		}
	}

	cpuLimit := devfileContainer.CpuLimit
	if cpuLimit == "" {
		cpuLimit = constants.SidecarDefaultCpuLimit
	}
	if cpuLimit != "" {
		cpuLimitQuantity, err := resource.ParseQuantity(cpuLimit)
		if err != nil {
			return nil, fmt.Errorf("failed to parse cpu limit %q: %w", cpuLimit, err)
		}
		limits[corev1.ResourceCPU] = cpuLimitQuantity
	}

	cpuReq := devfileContainer.CpuRequest
	if cpuReq == "" {
		cpuReq = constants.SidecarDefaultCpuRequest
	}
	if cpuReq != "" {
		cpuReqQuantity, err := resource.ParseQuantity(cpuReq)
		if err != nil {
			return nil, fmt.Errorf("failed to parse cpu request %q: %w", cpuReq, err)
		}
		requests[corev1.ResourceCPU] = cpuReqQuantity
	}

	if parsedCPULimit, ok := limits[corev1.ResourceCPU]; ok {
		if parsedCPUReq, ok := requests[corev1.ResourceCPU]; ok {
			if parsedCPULimit.Cmp(parsedCPUReq) < 0 {
				return nil, fmt.Errorf("container resources are invalid: CPU limit (%s) is less than request (%s)", cpuLimit, cpuReq)
			}
		}
	}

	return &corev1.ResourceRequirements{
		Limits:   limits,
		Requests: requests,
	}, nil
}

func devfileVolumeMountsToContainerVolumeMounts(devfileVolumeMounts []dw.VolumeMount) []corev1.VolumeMount {
	var volumeMounts []corev1.VolumeMount
	for _, vm := range devfileVolumeMounts {
		path := vm.Path
		if path == "" {
			// Devfile API spec: if path is unspecified, default is to use volume name
			path = fmt.Sprintf("/%s", vm.Name)
		}
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      vm.Name,
			MountPath: path,
		})
	}
	return volumeMounts
}

func devfileEnvToContainerEnv(componentName string, devfileEnvVars []dw.EnvVar) []corev1.EnvVar {
	var env = []corev1.EnvVar{
		{
			Name:  constants.DevWorkspaceComponentName,
			Value: componentName,
		},
	}

	for _, devfileEnv := range devfileEnvVars {
		env = append(env, corev1.EnvVar{
			Name:  devfileEnv.Name,
			Value: devfileEnv.Value,
		})
	}
	return env
}
