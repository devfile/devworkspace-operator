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

package annotate

import (
	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
)

// AddSourceAttributesForPlugin adds an attribute 'controller.devfile.io/imported-by=sourceID' to all elements of
// a plugin that support attributes.
func AddSourceAttributesForPlugin(sourceID string, plugin *dw.DevWorkspaceTemplateSpec) {
	for idx, component := range plugin.Components {
		if component.Attributes == nil {
			plugin.Components[idx].Attributes = attributes.Attributes{}
		}
		plugin.Components[idx].Attributes.PutString(PluginSourceAttribute, sourceID)
	}
	for idx, command := range plugin.Commands {
		if command.Attributes == nil {
			plugin.Commands[idx].Attributes = attributes.Attributes{}
		}
		plugin.Commands[idx].Attributes.PutString(PluginSourceAttribute, sourceID)
	}
	for idx, project := range plugin.Projects {
		if project.Attributes == nil {
			plugin.Projects[idx].Attributes = attributes.Attributes{}
		}
		plugin.Projects[idx].Attributes.PutString(PluginSourceAttribute, sourceID)
	}
	for idx, project := range plugin.StarterProjects {
		if project.Attributes == nil {
			plugin.Projects[idx].Attributes = attributes.Attributes{}
		}
		plugin.Projects[idx].Attributes.PutString(PluginSourceAttribute, sourceID)
	}
}
