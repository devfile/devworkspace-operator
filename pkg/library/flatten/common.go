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

import dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"

func DevWorkspaceIsFlattened(devworkspace *dw.DevWorkspaceTemplateSpec, contributions []dw.ComponentContribution) bool {
	if devworkspace == nil {
		return len(contributions) == 0
	}

	if devworkspace.Parent != nil {
		return false
	}

	if len(contributions) > 0 {
		return false
	}

	for _, component := range devworkspace.Components {
		if component.Plugin != nil {
			return false
		}
	}

	return true
}
