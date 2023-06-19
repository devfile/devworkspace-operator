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

package resources

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
)

// Takes a component and returns the resource requests and limits that it defines.
// If a resource request or limit is not defined within the component, it will
// not be populated in the corresponding corev1.ResourceList map.
// Returns an error if  a non-container component is passed to the function, or if an error
// occurs while parsing a resource limit or request.
func ParseResourcesFromComponent(component *dw.Component) (*corev1.ResourceRequirements, error) {
	if component.Container == nil {
		return nil, fmt.Errorf("attempted to parse resource requirements from a non-container component")
	}

	resources := &corev1.ResourceRequirements{
		Limits:   corev1.ResourceList{},
		Requests: corev1.ResourceList{},
	}

	memLimitStr := component.Container.MemoryLimit
	if memLimitStr != "" {
		memoryLimit, err := resource.ParseQuantity(memLimitStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse memory limit for container component %s: %w", component.Name, err)
		}
		resources.Limits[corev1.ResourceMemory] = memoryLimit
	}

	memRequestStr := component.Container.MemoryRequest
	if memRequestStr != "" {
		memoryRequest, err := resource.ParseQuantity(memRequestStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse memory request for container component %s: %w", component.Name, err)
		}
		resources.Requests[corev1.ResourceMemory] = memoryRequest
	}

	cpuLimitStr := component.Container.CpuLimit
	if cpuLimitStr != "" {
		cpuLimit, err := resource.ParseQuantity(cpuLimitStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse CPU limit for container component %s: %w", component.Name, err)
		}
		resources.Limits[corev1.ResourceCPU] = cpuLimit
	}

	cpuRequestStr := component.Container.CpuRequest
	if cpuRequestStr != "" {
		cpuRequest, err := resource.ParseQuantity(cpuRequestStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse CPU request for container component %s: %w", component.Name, err)
		}
		resources.Requests[corev1.ResourceCPU] = cpuRequest
	}

	return resources, nil
}

// Adds the resource limits and requests that are set in the component "toAdd" to "resources".
// Returns an error if the resources defined in toAdd could not be parsed.
//
// Note: only resources that are set in the function argument "resources" will be summed
// with the resources set in "toAdd".
// For example, if "resources" does not set a CPU limit but "toAdd" does set a CPU limit,
// the CPU limit of "resources" will remain unset.
func AddResourceRequirements(resources *corev1.ResourceRequirements, toAdd *dw.Component) error {
	componentResources, err := ParseResourcesFromComponent(toAdd)
	if err != nil {
		return err
	}

	for resourceName, limit := range resources.Limits {
		if componentLimit, ok := componentResources.Limits[resourceName]; ok {
			limit.Add(componentLimit)
			resources.Limits[resourceName] = limit
		}
	}

	for resourceName, request := range resources.Requests {
		if componentRequest, ok := componentResources.Requests[resourceName]; ok {
			request.Add(componentRequest)
			resources.Requests[resourceName] = request
		}
	}

	return nil
}

// Applies the given resource limits and requirements that are non-zero to the container component.
// If a resource limit or request has a value of zero, then the corresponding limit or request is not set
// in the container component's resource requirements.
func ApplyResourceRequirementsToComponent(container *dw.ContainerComponent, resources *corev1.ResourceRequirements) {
	memLimit := resources.Limits[corev1.ResourceMemory]
	if !memLimit.IsZero() {
		container.MemoryLimit = memLimit.String()
	}

	cpuLimit := resources.Limits[corev1.ResourceCPU]
	if !cpuLimit.IsZero() {
		container.CpuLimit = cpuLimit.String()
	}

	memRequest := resources.Requests[corev1.ResourceMemory]
	if !memRequest.IsZero() {
		container.MemoryRequest = memRequest.String()
	}

	cpuRequest := resources.Requests[corev1.ResourceCPU]
	if !cpuRequest.IsZero() {
		container.CpuRequest = cpuRequest.String()
	}
}

// ProcessResources checks that specified resources are valid (e.g. requests are less than limits) and supports
// un-setting resources that have default values by interpreting zero as "do not set"
func ProcessResources(resources *corev1.ResourceRequirements) (*corev1.ResourceRequirements, error) {
	result := resources.DeepCopy()

	if result.Limits.Memory().IsZero() {
		delete(result.Limits, corev1.ResourceMemory)
	}
	if result.Limits.Cpu().IsZero() {
		delete(result.Limits, corev1.ResourceCPU)
	}
	if result.Requests.Memory().IsZero() {
		delete(result.Requests, corev1.ResourceMemory)
	}
	if result.Requests.Cpu().IsZero() {
		delete(result.Requests, corev1.ResourceCPU)
	}

	memLimit, hasMemLimit := result.Limits[corev1.ResourceMemory]
	memRequest, hasMemRequest := result.Requests[corev1.ResourceMemory]
	if hasMemLimit && hasMemRequest && memRequest.Cmp(memLimit) > 0 {
		return result, fmt.Errorf("project clone memory request (%s) must be less than limit (%s)", memRequest.String(), memLimit.String())
	}

	cpuLimit, hasCPULimit := result.Limits[corev1.ResourceCPU]
	cpuRequest, hasCPURequest := result.Requests[corev1.ResourceCPU]
	if hasCPULimit && hasCPURequest && cpuRequest.Cmp(cpuLimit) > 0 {
		return result, fmt.Errorf("project clone CPU request (%s) must be less than limit (%s)", cpuRequest.String(), cpuLimit.String())
	}

	if len(result.Limits) == 0 {
		result.Limits = nil
	}
	if len(result.Requests) == 0 {
		result.Requests = nil
	}

	return result, nil
}
