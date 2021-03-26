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
}
