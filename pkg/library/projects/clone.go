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

// Package projects defines library functions for reconciling projects in a Devfile (i.e. cloning and maintaining state)
package projects

import (
	"fmt"
	"strings"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	devfileConstants "github.com/devfile/devworkspace-operator/pkg/library/constants"
	dwResources "github.com/devfile/devworkspace-operator/pkg/library/resources"
	corev1 "k8s.io/api/core/v1"

	"github.com/devfile/devworkspace-operator/internal/images"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

const (
	projectClonerContainerName = "project-clone"
)

type Options struct {
	Image      string
	PullPolicy corev1.PullPolicy
	Resources  *corev1.ResourceRequirements
	Env        []corev1.EnvVar
}

// ValidateAllProjectsvalidates that no two projects, dependentProjects or starterProjects (if one is selected) share
// the same name or cloned path
func ValidateAllProjects(workspace *dw.DevWorkspaceTemplateSpec) error {
	// Map of project names to project sources (project, dependentProject, starterProject)
	projectNames := map[string][]string{}
	// Map of project clone paths to project sources ("project <name>", "starterProject <name>", "dependentProject <name>")
	clonePaths := map[string][]string{}

	for idx, project := range workspace.Projects {
		projectNames[project.Name] = append(projectNames[project.Name], "projects")
		clonePath := GetClonePath(&workspace.Projects[idx])
		clonePaths[clonePath] = append(clonePaths[clonePath], fmt.Sprintf("project %s", project.Name))
	}

	for idx, project := range workspace.DependentProjects {
		projectNames[project.Name] = append(projectNames[project.Name], "dependentProjects")
		clonePath := GetClonePath(&workspace.DependentProjects[idx])
		clonePaths[clonePath] = append(clonePaths[clonePath], fmt.Sprintf("dependentProject %s", project.Name))
	}

	starterProject, err := GetStarterProject(workspace)
	if err != nil {
		return err
	}
	if starterProject != nil {
		projectNames[starterProject.Name] = append(projectNames[starterProject.Name], "starterProjects")
		// Starter projects do not have a clonePath field
		clonePaths[starterProject.Name] = append(clonePaths[starterProject.Name], fmt.Sprintf("starterProject %s", starterProject.Name))
	}

	for projectName, projectTypes := range projectNames {
		if len(projectTypes) > 1 {
			return fmt.Errorf("found multiple projects with the same name '%s' in: %s", projectName, strings.Join(projectTypes, ", "))
		}
	}
	for clonePath, projects := range clonePaths {
		if len(projects) > 1 {
			return fmt.Errorf("found multiple projects with the same clone path (%s): %s", clonePath, strings.Join(projects, ", "))
		}
	}

	return nil
}

func GetProjectCloneInitContainer(workspace *dw.DevWorkspaceTemplateSpec, options Options, proxyConfig *controllerv1alpha1.Proxy) (*corev1.Container, error) {
	starterProject, err := GetStarterProject(workspace)
	if err != nil {
		return nil, err
	}
	if len(workspace.Projects) == 0 && len(workspace.DependentProjects) == 0 && starterProject == nil {
		return nil, nil
	}
	if workspace.Attributes.GetString(constants.ProjectCloneAttribute, nil) == constants.ProjectCloneDisable {
		return nil, nil
	}
	if !hasContainerComponents(workspace) {
		// Avoid adding project-clone init container when DevWorkspace does not define any containers
		return nil, nil
	}

	var cloneImage string
	if options.Image != "" {
		cloneImage = options.Image
	} else {
		cloneImage = images.GetProjectCloneImage()
	}
	if cloneImage == "" {
		// Assume project clone is intentionally disabled if project clone image is not defined
		return nil, nil
	}

	resources := dwResources.FilterResources(options.Resources)
	if err := dwResources.ValidateResources(resources); err != nil {
		return nil, fmt.Errorf("invalid resources for project clone container: %w", err)
	}

	return &corev1.Container{
		Name:      projectClonerContainerName,
		Image:     cloneImage,
		Env:       options.Env,
		Resources: *resources,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      devfileConstants.ProjectsVolumeName,
				MountPath: constants.DefaultProjectsSourcesRoot,
			},
		},
		ImagePullPolicy: options.PullPolicy,
	}, nil
}

func GetStarterProject(workspace *dw.DevWorkspaceTemplateSpec) (*dw.StarterProject, error) {
	if !workspace.Attributes.Exists(constants.StarterProjectAttribute) {
		return nil, nil
	}
	var err error
	selectedStarterProject := workspace.Attributes.GetString(constants.StarterProjectAttribute, &err)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s attribute on workspace: %w", constants.StarterProjectAttribute, err)
	}
	for _, starterProject := range workspace.StarterProjects {
		if starterProject.Name == selectedStarterProject {
			starterProject := starterProject
			return &starterProject, nil
		}
	}
	return nil, fmt.Errorf("selected starter project %s not found in workspace starterProjects", selectedStarterProject)
}

// GetClonePath gets the correct clonePath for a project, given the semantics in devfile/api
func GetClonePath(project *dw.Project) string {
	if project.ClonePath != "" {
		return project.ClonePath
	}
	return project.Name
}

func hasContainerComponents(workspace *dw.DevWorkspaceTemplateSpec) bool {
	for _, component := range workspace.Components {
		if component.Container != nil {
			return true
		}
	}
	return false
}
