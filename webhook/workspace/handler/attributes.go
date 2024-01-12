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

package handler

import (
	"fmt"
	"strings"

	"github.com/devfile/devworkspace-operator/pkg/constants"

	dwv2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
)

// Checks whether the given DevWorkspace Template Spec has multiple container components with the
// controller.devfile.io/merge-contribution attribute set to true.
// If only a single container component has the controller.devfile.io/merge-contribution attribute set to true, nil is returned.
// If multiple container component have the controller.devfile.io/merge-contribution attribute set to true, or an error occurs
// while parsing the attribute, an error is returned.
func checkMultipleContainerContributionTargets(devWorkspaceSpec dwv2.DevWorkspaceTemplateSpec) error {
	var componentNames []string
	for _, component := range devWorkspaceSpec.Components {
		if component.Container == nil {
			// Ignore attribute on non-container components as it's not clear what this would mean
			continue
		}
		if component.Attributes.Exists(constants.MergeContributionAttribute) {
			var errHolder error
			if component.Attributes.GetBoolean(constants.MergeContributionAttribute, &errHolder) {
				componentNames = append(componentNames, component.Name)
			}

			if errHolder != nil {
				return fmt.Errorf("failed to parse %s attribute on component %s as true or false", constants.MergeContributionAttribute, component.Name)
			}
		}
	}

	if len(componentNames) > 1 {
		return fmt.Errorf("only a single component may have the %s attribute set to true. The following %d components have the %s attribute set to true: %s", constants.MergeContributionAttribute, len(componentNames), constants.MergeContributionAttribute, strings.Join(componentNames, ", "))
	}

	return nil
}
