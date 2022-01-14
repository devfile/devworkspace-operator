//
// Copyright (c) 2019-2022 Red Hat, Inc.
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
func handleMountSources(k8sContainer *corev1.Container, devfileContainer *dw.ContainerComponent, projects []dw.Project) {
	if !HasMountSources(devfileContainer) {
		return
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

	projectsSourcePath := getProjectSourcePath(projects)

	k8sContainer.Env = append(k8sContainer.Env, corev1.EnvVar{
		Name:  devfileConstants.ProjectsRootEnvVar,
		Value: sourceMapping,
	}, corev1.EnvVar{
		Name:  devfileConstants.ProjectsSourceEnvVar,
		Value: path.Join(sourceMapping, projectsSourcePath),
	})
}

// getProjectSourcePath gets the path, relative to PROJECTS_ROOT, that should be used for the PROJECT_SOURCE env var
func getProjectSourcePath(projects []dw.Project) string {
	projectPath := ""
	if len(projects) > 0 {
		firstProject := projects[0]
		if firstProject.ClonePath != "" {
			projectPath = firstProject.ClonePath
		} else {
			projectPath = firstProject.Name
		}
	}
	return projectPath
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
