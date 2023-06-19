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

package flatten

import (
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
)

// resolutionContextTree is a recursive structure representing information about the devworkspace that is
// lost when flattening
type resolutionContextTree struct {
	componentName   string
	importReference dw.ImportReference
	plugins         []*resolutionContextTree
	parentNode      *resolutionContextTree
}

func (t *resolutionContextTree) addPlugin(name string, plugin *dw.PluginComponent) *resolutionContextTree {
	newNode := &resolutionContextTree{
		componentName:   name,
		importReference: plugin.ImportReference,
		parentNode:      t,
	}
	t.plugins = append(t.plugins, newNode)
	return newNode
}

func (t *resolutionContextTree) hasCycle() error {
	var seenRefs []dw.ImportReference
	currNode := t
	for currNode.parentNode != nil {
		for _, seenRef := range seenRefs {
			if reflect.DeepEqual(seenRef, currNode.importReference) {
				return fmt.Errorf("DevWorkspace has an cycle in references: %s", formatImportCycle(t))
			}
		}
		seenRefs = append(seenRefs, currNode.importReference)
		currNode = currNode.parentNode
	}
	return nil
}

func formatImportCycle(end *resolutionContextTree) string {
	cycle := fmt.Sprintf("%s", end.componentName)
	for end.parentNode != nil {
		end = end.parentNode
		if end.parentNode == nil {
			end.componentName = "devworkspace"
		}
		cycle = fmt.Sprintf("%s -> %s", end.componentName, cycle)
	}
	return cycle
}

func parseResourcesFromComponent(component *dw.Component) (*corev1.ResourceRequirements, error) {
	if component.Container == nil {
		return nil, fmt.Errorf("attemped to parse resource requirements from a non-container component")
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

func addResourceRequirements(resources *corev1.ResourceRequirements, toAdd *dw.Component) error {
	componentResources, err := parseResourcesFromComponent(toAdd)
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

func applyResourceRequirementsToComponent(container *dw.ContainerComponent, resources *corev1.ResourceRequirements) {
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
