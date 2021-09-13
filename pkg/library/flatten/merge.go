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

package flatten

import (
	"fmt"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/utils/overriding"
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
	if !from.Ephemeral {
		into.Ephemeral = false
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
