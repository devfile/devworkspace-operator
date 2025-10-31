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

package home

import (
	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

// InferWorkspaceImage finds the first non-imported container component image in the
// flattened devfile template. This mirrors the selection rule used by the built-in
// persistent-home initializer to pick a "primary" workspace image. 
// If no such component exists, it returns an empty string.
func InferWorkspaceImage(dwTemplate *v1alpha2.DevWorkspaceTemplateSpec) string {
	for _, component := range dwTemplate.Components {
		if component.Container == nil {
			continue
		}
		pluginSource := component.Attributes.GetString(constants.PluginSourceAttribute, nil)
		if pluginSource == "" || pluginSource == "parent" {
			return component.Container.Image
		}
	}
	return ""
}
