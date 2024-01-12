//
// Copyright (c) 2019-2024 Red Hat, Inc.
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
	"strings"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	"github.com/devfile/api/v2/pkg/utils/overriding"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	dwResources "github.com/devfile/devworkspace-operator/pkg/library/resources"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"
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
	if from.Ephemeral == nil {
		into.Ephemeral = nil
	} else if !*from.Ephemeral {
		into.Ephemeral = pointer.Bool(false)
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

// needsContainerContributionMerge returns whether merging container contributions is necessary for this workspace. Merging
// is necessary if the following two conditions are met:
//
//   - At least one component has the container-contribution: true attribute.
//
//   - At least one component has the merge-contribution: true attribute OR there exists a container component that was not imported by a
//     plugin devworkspace.
//
// If the container-contribution or merge-contribution attribute are present but cannot be parsed as a bool, an error is returned.
// If multiple components have the merge-contribution: true attribute, an error is returned.
func needsContainerContributionMerge(flattenedSpec *dw.DevWorkspaceTemplateSpec) (bool, error) {
	hasContribution, hasTarget, explicitTarget := false, false, false
	var errHolder error
	for _, component := range flattenedSpec.Components {
		if component.Container == nil {
			// Ignore attribute on non-container components as it's not clear what this would mean
			continue
		}
		// Need to check existence before value to avoid potential KeyNotFoundError
		if component.Attributes.Exists(constants.ContainerContributionAttribute) {
			if component.Attributes.GetBoolean(constants.ContainerContributionAttribute, &errHolder) {
				hasContribution = true
			}
			if errHolder != nil {
				// Don't include error in message as it will be propagated to user and is not very clear (references Go unmarshalling)
				return false, fmt.Errorf("failed to parse %s attribute on component %s as true or false", constants.ContainerContributionAttribute, component.Name)
			}
		} else if component.Attributes.Exists(constants.MergeContributionAttribute) {
			// Explicit opt out case is handled here if the merge-contributions attribute is set to false
			if component.Attributes.GetBoolean(constants.MergeContributionAttribute, &errHolder) {
				if explicitTarget {
					return false, fmt.Errorf("multiple components have the %s attribute set to true. Only a single component may have the %s attribute set to true", constants.MergeContributionAttribute, constants.MergeContributionAttribute)
				}
				explicitTarget = true
				hasTarget = true
			}
			if errHolder != nil {
				return false, fmt.Errorf("failed to parse %s attribute on component %s as true or false", constants.MergeContributionAttribute, component.Name)
			}
		} else {
			pluginSource := component.Attributes.GetString(constants.PluginSourceAttribute, nil)
			if pluginSource == "" || pluginSource == "parent" {
				// First, non-imported container component is implicitly selected as a contribution target
				hasTarget = true
			}
		}
	}
	return hasContribution && hasTarget, nil
}

func mergeContainerContributions(flattenedSpec *dw.DevWorkspaceTemplateSpec, defaultResources *corev1.ResourceRequirements) error {
	var contributions []dw.Component
	contributionNameSet := map[string]bool{}
	for _, component := range flattenedSpec.Components {
		if component.Container != nil && component.Attributes.GetBoolean(constants.ContainerContributionAttribute, nil) {
			contributions = append(contributions, component)
			contributionNameSet[component.Name] = true
		}
	}

	targetComponentName, err := findMergeTarget(flattenedSpec)
	if err != nil {
		return err
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
		} else if component.Name == targetComponentName && !mergeDone {
			mergedComponent, err := mergeContributionsInto(&component, contributions, defaultResources)
			if err != nil {
				return fmt.Errorf("failed to merge container contributions: %w", err)
			}
			newComponents = append(newComponents, *mergedComponent)
			mergeDone = true
		} else {
			newComponents = append(newComponents, component)
		}
	}

	if mergeDone {
		if err := updateCommandTargetsAfterMerge(flattenedSpec, targetComponentName, contributionNameSet); err != nil {
			return err
		}
		flattenedSpec.Components = newComponents
	}

	return nil
}

// Finds a component that is a suitable merge target for container contributions and returns its name.
// The following rules are followed when finding a merge target:
//
//   - A container component that has the merge-contribution: true attribute will automatically be selected as a merge target.
//
//   - A container component that has the merge-contribution: false attribute will be never be selected as a merge target.
//
//   - Otherwise, the first container component found that was not imported by a plugin devworkspace
//     will be selected as a merge target.
//
// If no suitable merge target is found, an error is returned.
func findMergeTarget(flattenedSpec *dw.DevWorkspaceTemplateSpec) (mergeTargetComponentName string, err error) {
	firstComponent := ""
	for _, component := range flattenedSpec.Components {
		if component.Container == nil {
			continue
		}

		if component.Attributes.Exists(constants.MergeContributionAttribute) {
			// Check for explicit merge contributtion attribute
			if component.Attributes.GetBoolean(constants.MergeContributionAttribute, nil) {
				return component.Name, nil
			}
			// Don't select components that opt out as a merge contribution target
			continue
		}

		// The target must not have been imported by a plugin.
		pluginSource := component.Attributes.GetString(constants.PluginSourceAttribute, nil)
		if pluginSource != "" && pluginSource != "parent" {
			continue
		}

		// There might be other components that explicitly opt in as a merge target,
		// so don't return immediately
		if firstComponent == "" {
			firstComponent = component.Name
		}
	}

	if firstComponent != "" {
		return firstComponent, nil
	}

	return "", fmt.Errorf("couldn't find any merge contribution target component")
}

func mergeContributionsInto(mergeInto *dw.Component, contributions []dw.Component, defaultResources *corev1.ResourceRequirements) (*dw.Component, error) {
	if mergeInto == nil || mergeInto.Container == nil {
		return nil, fmt.Errorf("attempting to merge container contributions into a non-container component")
	}
	totalResources, err := dwResources.ParseResourcesFromComponent(mergeInto)
	if err != nil {
		return nil, err
	}
	totalResources = dwResources.ApplyDefaults(totalResources, defaultResources)

	// We don't want to reimplement the complexity of a strategic merge here, so we set up a fake plugin override
	// and use devfile/api overriding functionality. For specific fields that have to be handled specially (memory
	// and cpu limits, we compute the value separately and set it at the end
	var toMerge []dw.ComponentPluginOverride
	// Store names of original plugins to allow us to generate the merged-contributions attribute
	var mergedComponentNames []string
	for _, component := range contributions {
		if component.Container == nil {
			return nil, fmt.Errorf("attempting to merge container contribution from a non-container component")
		}
		// Set name to match target component so that devfile/api override functionality will apply it correctly
		component.Name = mergeInto.Name
		// Unset image to avoid overriding the default image
		component.Container.Image = ""
		// Store original source attribute's value and remove from component
		if component.Attributes.Exists(constants.PluginSourceAttribute) {
			mergedComponentNames = append(mergedComponentNames, component.Attributes.GetString(constants.PluginSourceAttribute, nil))
			delete(component.Attributes, constants.PluginSourceAttribute)
		}
		componentResources, err := dwResources.ParseResourcesFromComponent(&component)
		if err != nil {
			return nil, err
		}
		totalResources = dwResources.AddResourceRequirements(totalResources, componentResources)

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
	dwResources.ApplyResourceRequirementsToComponent(mergedComponent.Container, totalResources)

	if mergedComponent.Attributes == nil {
		mergedComponent.Attributes = attributes.Attributes{}
	}
	mergedComponent.Attributes.PutString(constants.MergedContributionsAttribute, strings.Join(mergedComponentNames, ","))
	delete(mergedComponent.Attributes, constants.MergeContributionAttribute)
	delete(mergedComponent.Attributes, constants.ContainerContributionAttribute)

	return &mergedComponent, nil
}

func updateCommandTargetsAfterMerge(flattenedSpec *dw.DevWorkspaceTemplateSpec, mergedName string, contributionNames map[string]bool) error {
	var newCommands []dw.Command
	for _, command := range flattenedSpec.Commands {
		switch {
		case command.Exec != nil:
			if contributionNames[command.Exec.Component] {
				command.Exec.Component = mergedName
			}
		case command.Apply != nil:
			if contributionNames[command.Apply.Component] {
				return fmt.Errorf("apply command %s uses container contribution %s as component", command.Id, command.Apply.Component)
			}
		}
		newCommands = append(newCommands, command)
	}
	flattenedSpec.Commands = newCommands
	return nil
}
