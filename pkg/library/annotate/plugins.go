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

package annotate

import (
	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"

	"github.com/devfile/devworkspace-operator/pkg/constants"
)

// AddSourceAttributesForTemplate adds an attribute 'controller.devfile.io/imported-by=sourceID' to all elements of
// a plugin that support attributes.
func AddSourceAttributesForTemplate(sourceID string, template *dw.DevWorkspaceTemplateSpec) {
	for idx, component := range template.Components {
		if component.Attributes == nil {
			template.Components[idx].Attributes = attributes.Attributes{}
		}
		template.Components[idx].Attributes.PutString(constants.PluginSourceAttribute, sourceID)
	}
	for idx, command := range template.Commands {
		if command.Attributes == nil {
			template.Commands[idx].Attributes = attributes.Attributes{}
		}
		template.Commands[idx].Attributes.PutString(constants.PluginSourceAttribute, sourceID)
	}
	for idx, project := range template.Projects {
		if project.Attributes == nil {
			template.Projects[idx].Attributes = attributes.Attributes{}
		}
		template.Projects[idx].Attributes.PutString(constants.PluginSourceAttribute, sourceID)
	}
	for idx, project := range template.StarterProjects {
		if project.Attributes == nil {
			template.StarterProjects[idx].Attributes = attributes.Attributes{}
		}
		template.StarterProjects[idx].Attributes.PutString(constants.PluginSourceAttribute, sourceID)
	}
	for idx, project := range template.DependentProjects {
		if project.Attributes == nil {
			template.DependentProjects[idx].Attributes = attributes.Attributes{}
		}
		template.DependentProjects[idx].Attributes.PutString(constants.PluginSourceAttribute, sourceID)
	}
}
