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

package container

import (
	devworkspace "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	corev1 "k8s.io/api/core/v1"

	"github.com/devfile/devworkspace-operator/pkg/constants"
	devfileConstants "github.com/devfile/devworkspace-operator/pkg/library/constants"
)

// HasMountSources evaluates whether project sources should be mounted in the given container component.
// MountSources is by default true for non-plugin components, unless they have dedicatedPod set
// TODO:
// - Support dedicatedPod field
// - Find way to track is container component comes from plugin
func HasMountSources(devfileContainer *devworkspace.ContainerComponent) bool {
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
func AnyMountSources(devfileComponents []devworkspace.Component) bool {
	for _, component := range devfileComponents {
		if component.Container != nil && HasMountSources(component.Container) {
			return true
		}
	}
	return false
}

// handleMountSources adds a volumeMount to a container if the corresponding devfile container has
// mountSources enabled.
func handleMountSources(k8sContainer *corev1.Container, devfileContainer *devworkspace.ContainerComponent) {
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
	k8sContainer.Env = append(k8sContainer.Env, corev1.EnvVar{
		Name:  devfileConstants.ProjectsRootEnvVar,
		Value: sourceMapping,
	}, corev1.EnvVar{
		Name:  devfileConstants.ProjectsSourceEnvVar,
		Value: sourceMapping, // TODO: Unclear how this should be handled in case of multiple projects
	})
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
