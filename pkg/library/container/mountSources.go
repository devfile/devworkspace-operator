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

package container

import (
	"path"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	corev1 "k8s.io/api/core/v1"

	"github.com/devfile/devworkspace-operator/pkg/constants"
	devfileConstants "github.com/devfile/devworkspace-operator/pkg/library/constants"
	projectslib "github.com/devfile/devworkspace-operator/pkg/library/projects"
)

// HasMountSources evaluates whether project sources should be mounted in the given container component.
// MountSources is by default true for non-plugin components, unless they have dedicatedPod set
// TODO:
// - Support dedicatedPod field
// - Find way to track is container component comes from plugin
func HasMountSources(devfileContainer *dw.ContainerComponent) bool {
	var mountSources bool
	if devfileContainer.MountSources == nil {
		mountSources = true
	} else {
		mountSources = *devfileContainer.MountSources
	}
	return mountSources
}

// AnyMountSources checks HasMountSources for each container component in a devfile. If a component in the slice
// is not a ContainerComponent, it is ignored.
func AnyMountSources(devfileComponents []dw.Component) bool {
	for _, component := range devfileComponents {
		if component.Container != nil && HasMountSources(component.Container) {
			return true
		}
	}
	return false
}

// handleMountSources adds a volumeMount to a container if the corresponding devfile container has
// mountSources enabled.
func handleMountSources(k8sContainer *corev1.Container, devfileContainer *dw.ContainerComponent, workspace *dw.DevWorkspaceTemplateSpec) error {
	if !HasMountSources(devfileContainer) {
		return nil
	}
	var sourceMapping string
	if vm := getProjectsVolumeMount(k8sContainer); vm != nil {
		// Container already mounts projects volume; need to set env vars according to mountPath
		// TODO: see issue https://github.com/devfile/api/issues/290
		sourceMapping = vm.MountPath
	} else {
		sourceMapping = devfileContainer.SourceMapping
		if sourceMapping == "" {
			// Sanity check -- this value should be defaulted to `/projects` but may not be
			// if struct was not processed by k8s
			sourceMapping = constants.DefaultProjectsSourcesRoot
		}
		k8sContainer.VolumeMounts = append(k8sContainer.VolumeMounts, corev1.VolumeMount{
			Name:      devfileConstants.ProjectsVolumeName,
			MountPath: sourceMapping,
		})
	}

	projectsSourcePath, err := getProjectSourcePath(workspace)
	if err != nil {
		return err
	}

	k8sContainer.Env = append(k8sContainer.Env, corev1.EnvVar{
		Name:  devfileConstants.ProjectsRootEnvVar,
		Value: sourceMapping,
	}, corev1.EnvVar{
		Name:  devfileConstants.ProjectsSourceEnvVar,
		Value: path.Join(sourceMapping, projectsSourcePath),
	})

	return nil
}

// getProjectSourcePath gets the path, relative to PROJECTS_ROOT, that should be used for the PROJECT_SOURCE env var.
// Returns an error if there was a problem retrieving the selected starter project.
//
// The project source path is determined based on the following priorities:
//
//  1. If the workspace has at least one regular project, the first one will be selected.
//     If the first project has a clone path, it will be used, otherwise the project's name will be used as the project source path.
//
//  2. If the workspace has a starter project that is selected, its name will be used as the project source path.
//
//  3. If the workspace has any dependentProjects, the first one will be selected.
//
//  4. Otherwise, the returned project source path will be an empty string.
func getProjectSourcePath(workspace *dw.DevWorkspaceTemplateSpec) (string, error) {
	projects := workspace.Projects
	// If there are any projects, return the first one's clone path
	if len(projects) > 0 {
		return projectslib.GetClonePath(&projects[0]), nil
	}

	// No projects, check if we have a selected starter project
	selectedStarterProject, err := projectslib.GetStarterProject(workspace)
	if err != nil {
		return "", err
	} else if selectedStarterProject != nil {
		// Starter projects do not allow specifying a clone path, so use the name
		return selectedStarterProject.Name, nil
	}

	// Finally, check if there are any dependent projects
	if len(workspace.DependentProjects) > 0 {
		return projectslib.GetClonePath(&workspace.DependentProjects[0]), nil
	}

	return "", nil
}

// getProjectsVolumeMount returns the projects volumeMount in a container, if it is defined; if it does not exist,
// returns nil.
func getProjectsVolumeMount(k8sContainer *corev1.Container) *corev1.VolumeMount {
	for _, vm := range k8sContainer.VolumeMounts {
		if vm.Name == devfileConstants.ProjectsVolumeName {
			return &vm
		}
	}
	return nil
}
