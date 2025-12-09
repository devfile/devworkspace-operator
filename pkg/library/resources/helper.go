//
// Copyright (c) 2019-2025 Red Hat, Inc.
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
//
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
// Returns an error if the "resources" defined in "toAdd" could not be parsed.
//
// Note: only resources that are set in the argument "resources" will be summed with the resources set in "toAdd".
// For example, if "resources" does not set a CPU limit but "toAdd" does set a CPU limit,
// the CPU limit of "resources" will remain unset.
func AddResourceRequirements(resources, toAdd *corev1.ResourceRequirements) *corev1.ResourceRequirements {
	result := resources.DeepCopy()

	for resourceName, limit := range resources.Limits {
		if componentLimit, ok := toAdd.Limits[resourceName]; ok {
			limit.Add(componentLimit)
			result.Limits[resourceName] = limit
		}
	}

	for resourceName, request := range resources.Requests {
		if componentRequest, ok := toAdd.Requests[resourceName]; ok {
			request.Add(componentRequest)
			result.Requests[resourceName] = request
		}
	}

	return result
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

// FilterResources removes zero values from a corev1.ResourceRequirements in order to allow explicitly defining
// "do not set a limit/request" for a resource. Any request/limit that has a zero value is removed from the returned
// corev1.ResourceRequirements.
func FilterResources(resources *corev1.ResourceRequirements) *corev1.ResourceRequirements {
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

	if len(result.Limits) == 0 {
		result.Limits = nil
	}
	if len(result.Requests) == 0 {
		result.Requests = nil
	}

	return result
}

func ApplyDefaults(resources, defaults *corev1.ResourceRequirements) *corev1.ResourceRequirements {
	result := resources.DeepCopy()
	if defaults == nil {
		return result
	}

	// Set default limits if not present
	for resourceName, quantity := range defaults.Limits {
		if result.Limits == nil {
			result.Limits = corev1.ResourceList{}
		}
		if _, ok := result.Limits[resourceName]; !ok && !quantity.IsZero() {
			result.Limits[resourceName] = quantity
		}
	}
	// Set default requests if not present
	for resourceName, quantity := range defaults.Requests {
		if result.Requests == nil {
			result.Requests = corev1.ResourceList{}
		}
		if _, ok := result.Requests[resourceName]; !ok && !quantity.IsZero() {
			result.Requests[resourceName] = quantity
		}
	}

	// Edge cases: we don't want the defaults we apply to result in an invalid resources (if e.g. the default
	// request is greater than the defined limit). In this case, we use the minimum (maximum) limit (request)
	// to ensure the result is still valid
	memLimit := result.Limits[corev1.ResourceMemory]
	memRequest := result.Requests[corev1.ResourceMemory]
	if !memLimit.IsZero() && !memRequest.IsZero() && memLimit.Cmp(memRequest) < 0 {
		originalMemLimit := resources.Limits[corev1.ResourceMemory]
		originalMemRequest := resources.Requests[corev1.ResourceMemory]
		switch {
		case originalMemLimit.IsZero() && !originalMemRequest.IsZero(): // The memory limit from default is smaller than the provided request
			result.Limits[corev1.ResourceMemory] = originalMemRequest
		case !originalMemLimit.IsZero() && originalMemRequest.IsZero(): // The memory request from default is greater than the provided limit
			result.Requests[corev1.ResourceMemory] = originalMemLimit
		default: // Invalid resources is not a result of applying defaults, do nothing
			break
		}
	}

	cpuLimit := result.Limits[corev1.ResourceCPU]
	cpuRequest := result.Requests[corev1.ResourceCPU]
	if !cpuLimit.IsZero() && !cpuRequest.IsZero() && cpuLimit.Cmp(cpuRequest) < 0 {
		originalCPULimit := resources.Limits[corev1.ResourceCPU]
		originalCPURequest := resources.Requests[corev1.ResourceCPU]
		switch {
		case originalCPULimit.IsZero() && !originalCPURequest.IsZero(): // The CPU limit from default is smaller than the provided request
			result.Limits[corev1.ResourceCPU] = originalCPURequest
		case !originalCPULimit.IsZero() && originalCPURequest.IsZero(): // The CPU request from default is greater than the provided limit
			result.Requests[corev1.ResourceCPU] = originalCPULimit
		default: // Invalid resources is not a result of applying defaults, do nothing
			break
		}
	}

	return result
}

func ApplyCaps(resources, caps *corev1.ResourceRequirements) *corev1.ResourceRequirements {
	result := resources.DeepCopy()
	if caps == nil {
		return result
	}

	// Apply caps limits as maximum values (use the smaller of existing and caps)
	for resourceName, capLimit := range caps.Limits {
		if capLimit.IsZero() {
			continue
		}
		if result.Limits == nil {
			result.Limits = corev1.ResourceList{}
		}
		existingLimit, hasExisting := result.Limits[resourceName]
		if !hasExisting || existingLimit.IsZero() {
			// No existing limit, use caps as maximum
			result.Limits[resourceName] = capLimit
		} else if existingLimit.Cmp(capLimit) > 0 {
			// Existing limit is higher than caps, apply caps maximum
			result.Limits[resourceName] = capLimit
		}
		// Otherwise, keep existing limit (it's already lower than or equal to caps)
	}

	// Apply caps requests as maximum values (use the smaller of existing and caps)
	for resourceName, capRequest := range caps.Requests {
		if capRequest.IsZero() {
			continue
		}
		if result.Requests == nil {
			result.Requests = corev1.ResourceList{}
		}
		existingRequest, hasExisting := result.Requests[resourceName]
		if !hasExisting || existingRequest.IsZero() {
			// No existing request, use caps as maximum
			result.Requests[resourceName] = capRequest
		} else if existingRequest.Cmp(capRequest) > 0 {
			// Existing request is higher than caps, apply caps maximum
			result.Requests[resourceName] = capRequest
		}
		// Otherwise, keep existing request (it's already lower than or equal to caps)
	}

	// Edge cases: after applying caps, we might create invalid resources (limit < request).
	// We need to adjust to ensure the result is still valid.
	memLimit := result.Limits[corev1.ResourceMemory]
	memRequest := result.Requests[corev1.ResourceMemory]
	if !memLimit.IsZero() && !memRequest.IsZero() && memLimit.Cmp(memRequest) < 0 {
		capMemLimit := caps.Limits[corev1.ResourceMemory]
		capMemRequest := caps.Requests[corev1.ResourceMemory]
		switch {
		case !capMemLimit.IsZero() && capMemRequest.IsZero():
			// Cap limit caused the issue, adjust request down to match limit
			result.Requests[corev1.ResourceMemory] = capMemLimit
		case capMemLimit.IsZero() && !capMemRequest.IsZero():
			// Cap request as maximum shouldn't cause limit < request, but adjust request down to match limit
			result.Requests[corev1.ResourceMemory] = memLimit
		default:
			// Both caps or neither caps - use caps limit for both to ensure validity
			if !capMemLimit.IsZero() {
				result.Limits[corev1.ResourceMemory] = capMemLimit
				result.Requests[corev1.ResourceMemory] = capMemLimit
			} else {
				result.Requests[corev1.ResourceMemory] = memLimit
			}
		}
	}

	cpuLimit := result.Limits[corev1.ResourceCPU]
	cpuRequest := result.Requests[corev1.ResourceCPU]
	if !cpuLimit.IsZero() && !cpuRequest.IsZero() && cpuLimit.Cmp(cpuRequest) < 0 {
		capCPULimit := caps.Limits[corev1.ResourceCPU]
		capCPURequest := caps.Requests[corev1.ResourceCPU]
		switch {
		case !capCPULimit.IsZero() && capCPURequest.IsZero():
			// Cap limit caused the issue, adjust request down to match limit
			result.Requests[corev1.ResourceCPU] = capCPULimit
		case capCPULimit.IsZero() && !capCPURequest.IsZero():
			// Cap request as maximum shouldn't cause limit < request, but adjust request down to match limit
			result.Requests[corev1.ResourceCPU] = cpuLimit
		default:
			// Both caps or neither caps - use caps limit for both to ensure validity
			if !capCPULimit.IsZero() {
				result.Limits[corev1.ResourceCPU] = capCPULimit
				result.Requests[corev1.ResourceCPU] = capCPULimit
			} else {
				result.Requests[corev1.ResourceCPU] = cpuLimit
			}
		}
	}

	return result
}

// ValidateResources validates that a corev1.ResourceRequirements is valid, i.e. that (if specified), limits are greater than or equal to requests
func ValidateResources(resources *corev1.ResourceRequirements) error {
	memLimit, hasMemLimit := resources.Limits[corev1.ResourceMemory]
	memRequest, hasMemRequest := resources.Requests[corev1.ResourceMemory]
	if hasMemLimit && hasMemRequest && memRequest.Cmp(memLimit) > 0 {
		return fmt.Errorf("memory request (%s) must be less than or equal to limit (%s)", memRequest.String(), memLimit.String())
	}

	cpuLimit, hasCPULimit := resources.Limits[corev1.ResourceCPU]
	cpuRequest, hasCPURequest := resources.Requests[corev1.ResourceCPU]
	if hasCPULimit && hasCPURequest && cpuRequest.Cmp(cpuLimit) > 0 {
		return fmt.Errorf("CPU request (%s) must be less than or equal to limit (%s)", cpuRequest.String(), cpuLimit.String())
	}

	return nil
}
