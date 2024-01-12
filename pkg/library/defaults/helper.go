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

package defaults

import (
	"github.com/devfile/devworkspace-operator/pkg/common"
)

// Overwrites the content of the workspace's Template Spec with the workspace config's Template Spec,
// with the exception of the workspace's projects.
// If the workspace's Template Spec defined any projects, they are preserved.
func ApplyDefaultTemplate(workspace *common.DevWorkspaceWithConfig) {
	if workspace.Config.Workspace.DefaultTemplate == nil {
		return
	}
	defaultCopy := workspace.Config.Workspace.DefaultTemplate.DeepCopy()
	originalProjects := workspace.Spec.Template.Projects
	originalDependentProjects := workspace.Spec.Template.DependentProjects
	workspace.Spec.Template.DevWorkspaceTemplateSpecContent = *defaultCopy
	workspace.Spec.Template.Projects = append(workspace.Spec.Template.Projects, originalProjects...)
	workspace.Spec.Template.DependentProjects = append(workspace.Spec.Template.DependentProjects, originalDependentProjects...)
}

func NeedsDefaultTemplate(workspace *common.DevWorkspaceWithConfig) bool {
	return len(workspace.Spec.Template.Components) == 0 && workspace.Config.Workspace.DefaultTemplate != nil
}
