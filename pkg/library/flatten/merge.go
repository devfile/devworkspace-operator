//
// Copyright (c) 2019-2022 Red Hat, Inc.
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
	"encoding/json"
	"fmt"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/utils/overriding"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"k8s.io/apimachinery/pkg/api/resource"
)

// mergeDevWorkspaceElements merges elements that are duplicated between the DevWorkspace, its parent, and its plugins
// where appropriate in order to avoid errors when merging elements via the devfile/api methods. Currently, only
// volumes are merged.
func mergeDevWorkspaceElements(main, parent *dw.DevWorkspaceTemplateSpecContent, plugins ...*dw.DevWorkspaceTemplateSpecContent) (*dw.DevWorkspaceTemplateSpecContent, error) {
	if err := mergeVolumeComponents(main, parent, plugins...); err != nil {
		return nil, fmt.Errorf("failed to merge DevWorkspace volumes: %w", err)
	}
	mergedDevWorkspaceTemplateSpec, err := overriding.MergeDevWorkspaceTemplateSpec(main, parent, plugins...)
	if err != nil {
		return nil, fmt.Errorf("failed to merge DevWorkspace parents/plugins: %w", err)
	}
	return mergedDevWorkspaceTemplateSpec, nil
}

// mergeVolumeComponents merges volume components sharing the same name according to the following rules
// * If a volume is defined in main and duplicated in parent/plugins, the copy in parent/plugins is removed
// * If a volume is defined in parent and duplicated in plugins, the copy in plugins is removed
// * If a volume is defined in multiple plugins, all but the first definition is removed
// * If a volume is defined as persistent, all duplicates will be persistent
// * If duplicate volumes set a size, the larger size will be used.
// Following the invocation of this function, there are no duplicate volumes defined across the main devworkspace, its
// parent, and its plugins.
func mergeVolumeComponents(main, parent *dw.DevWorkspaceTemplateSpecContent, plugins ...*dw.DevWorkspaceTemplateSpecContent) error {
	volumeComponents := map[string]dw.Component{}
	for _, component := range main.Components {
		if component.Volume == nil {
			continue
		}
		if _, exists := volumeComponents[component.Name]; exists {
			return fmt.Errorf("duplicate volume found in devfile: %s", component.Name)
		}
		volumeComponents[component.Name] = component
	}

	mergeVolumeComponents := func(spec *dw.DevWorkspaceTemplateSpecContent) error {
		var newComponents []dw.Component
		for _, component := range spec.Components {
			if component.Volume == nil {
				newComponents = append(newComponents, component)
				continue
			}
			if existingVol, exists := volumeComponents[component.Name]; exists {
				if err := mergeVolume(existingVol.Volume, component.Volume); err != nil {
					return err
				}
			} else {
				newComponents = append(newComponents, component)
				volumeComponents[component.Name] = component
			}
		}
		spec.Components = newComponents
		return nil
	}
	if err := mergeVolumeComponents(parent); err != nil {
		return err
	}

	for _, plugin := range plugins {
		if err := mergeVolumeComponents(plugin); err != nil {
			return err
		}
	}

	return nil
}

func mergeVolume(into, from *dw.VolumeComponent) error {
	// If the new volume is persistent, make the original persistent
	boolFalse := false
	if from.Ephemeral == nil {
		into.Ephemeral = nil
	} else if !*from.Ephemeral {
		into.Ephemeral = &boolFalse
	}
	intoSize := into.Size
	if intoSize == "" {
		intoSize = "0"
	}
	intoSizeQty, err := resource.ParseQuantity(intoSize)
	if err != nil {
		return err
	}
	fromSize := from.Size
	if fromSize == "" {
		fromSize = "0"
	}
	fromSizeQty, err := resource.ParseQuantity(fromSize)
	if err != nil {
		return err
	}
	if fromSizeQty.Cmp(intoSizeQty) > 0 {
		into.Size = from.Size
	}
	return nil
}

func needsContainerContributionMerge(flattenedSpec *dw.DevWorkspaceTemplateSpec) bool {
	hasContribution, hasTarget := false, false
	for _, component := range flattenedSpec.Components {
		if component.Container == nil {
			// Ignore attribute on non-container components as it's not clear what this would mean
			continue
		}
		if component.Attributes.GetBoolean(constants.ContainerContributionAttribute, nil) {
			hasContribution = true
		}
		if component.Attributes.GetBoolean(constants.MergeContributionAttribute, nil) {
			hasTarget = true
		}
	}
	return hasContribution && hasTarget
}

func mergeContainerContributions(flattenedSpec *dw.DevWorkspaceTemplateSpec) error {
	var contributions []dw.Component
	for _, component := range flattenedSpec.Components {
		if component.Container != nil && component.Attributes.GetBoolean(constants.ContainerContributionAttribute, nil) {
			contributions = append(contributions, component)
		}
	}

	var newComponents []dw.Component
	mergeDone := false
	for _, component := range flattenedSpec.Components {
		if component.Container == nil {
			newComponents = append(newComponents, component)
			continue
		}
		if component.Attributes.GetBoolean(constants.ContainerContributionAttribute, nil) {
			// drop contributions from updated list as they will be merged
			continue
		} else if component.Attributes.GetBoolean(constants.MergeContributionAttribute, nil) && !mergeDone {
			mergedComponent, err := mergeContributionsInto(&component, contributions)
			if err != nil {
				return fmt.Errorf("failed to merge container contributions: %w", err)
			}
			delete(mergedComponent.Attributes, constants.ContainerContributionAttribute)
			newComponents = append(newComponents, *mergedComponent)
			mergeDone = true
		} else {
			newComponents = append(newComponents, component)
		}
	}

	if mergeDone {
		flattenedSpec.Components = newComponents
	}

	return nil
}

func mergeContributionsInto(mergeInto *dw.Component, contributions []dw.Component) (*dw.Component, error) {
	if mergeInto == nil || mergeInto.Container == nil {
		return nil, fmt.Errorf("attempting to merge container contributions into a non-container component")
	}
	totalResources, err := parseResourcesFromComponent(mergeInto)
	if err != nil {
		return nil, err
	}

	// We don't want to reimplement the complexity of a strategic merge here, so we set up a fake plugin override
	// and use devfile/api overriding functionality. For specific fields that have to be handled specifically (memory
	// and cpu limits, we compute the value separately and set it at the end
	var toMerge []dw.ComponentPluginOverride
	for _, component := range contributions {
		if component.Container == nil {
			return nil, fmt.Errorf("attempting to merge container contribution from a non-container component")
		}
		// Set name to match target component so that devfile/api override functionality will apply it correctly
		component.Name = mergeInto.Name
		// Unset image to avoid overriding the default image
		component.Container.Image = ""
		if err := addResourceRequirements(totalResources, &component); err != nil {
			return nil, err
		}
		component.Container.MemoryLimit = ""
		component.Container.MemoryRequest = ""
		component.Container.CpuLimit = ""
		component.Container.CpuRequest = ""
		// Workaround to convert dw.Component into dw.ComponentPluginOverride: marshal to json, and unmarshal to a different type
		// This works since plugin overrides are generated from components, with the difference being that all fields are optional
		componentPluginOverride := dw.ComponentPluginOverride{}
		tempJSONBytes, err := json.Marshal(component)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(tempJSONBytes, &componentPluginOverride); err != nil {
			return nil, err
		}
		toMerge = append(toMerge, componentPluginOverride)
	}

	tempSpecContent := &dw.DevWorkspaceTemplateSpecContent{
		Components: []dw.Component{
			*mergeInto,
		},
	}

	mergedSpecContent, err := overriding.OverrideDevWorkspaceTemplateSpec(tempSpecContent, dw.PluginOverrides{
		Components: toMerge,
	})
	if err != nil {
		return nil, err
	}

	mergedComponent := mergedSpecContent.Components[0]
	applyResourceRequirementsToComponent(mergedComponent.Container, totalResources)

	return &mergedComponent, nil
}
