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

// Package container contains library functions for converting DevWorkspace Container components to Kubernetes
// components
//
// TODO:
//   - Devfile API spec is unclear on how mountSources should be handled -- mountPath is assumed to be /projects
//     and volume name is assumed to be "projects"
//     see issues:
//   - https://github.com/devfile/api/issues/290
//   - https://github.com/devfile/api/issues/291
package container

import (
	"fmt"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/library/flatten"
	"github.com/devfile/devworkspace-operator/pkg/library/lifecycle"
	"github.com/devfile/devworkspace-operator/pkg/library/overrides"
	corev1 "k8s.io/api/core/v1"
)

// GetKubeContainersFromDevfile converts container components in a DevWorkspace into Kubernetes containers.
// If a DevWorkspace container is an init container (i.e. is bound to a preStart event), it will be returned as an
// init container.
//
// This function also provisions volume mounts on containers as follows:
// - Container component's volume mounts are provisioned with the mount path and name specified in the devworkspace
// However, no Volumes are added to the returned PodAdditions at this stage; the volumeMounts above are expected to be
// rewritten as Volumes are added to PodAdditions, in order to support e.g. using one PVC to hold all volumes
//
// Note: Requires DevWorkspace to be flattened (i.e. the DevWorkspace contains no Parent or Components of type Plugin)
func GetKubeContainersFromDevfile(workspace *dw.DevWorkspaceTemplateSpec, securityContext *corev1.SecurityContext, pullPolicy string, defaultResources *corev1.ResourceRequirements) (*v1alpha1.PodAdditions, error) {
	if !flatten.DevWorkspaceIsFlattened(workspace, nil) {
		return nil, fmt.Errorf("devfile is not flattened")
	}
	podAdditions := &v1alpha1.PodAdditions{}

	initComponents, mainComponents, err := lifecycle.GetInitContainers(workspace.DevWorkspaceTemplateSpecContent)
	if err != nil {
		return nil, err
	}

	for _, component := range mainComponents {
		if component.Container == nil {
			continue
		}
		k8sContainer, err := convertContainerToK8s(component, securityContext, pullPolicy, defaultResources)
		if err != nil {
			return nil, err
		}
		if err := handleMountSources(k8sContainer, component.Container, workspace); err != nil {
			return nil, err
		}
		if overrides.NeedsContainerOverride(&component) {
			patchedContainer, err := overrides.ApplyContainerOverrides(&component, k8sContainer)
			if err != nil {
				return nil, err
			}
			k8sContainer = patchedContainer
		}
		podAdditions.Containers = append(podAdditions.Containers, *k8sContainer)
	}

	if err := lifecycle.AddPostStartLifecycleHooks(workspace, podAdditions.Containers); err != nil {
		return nil, err
	}

	if err := lifecycle.AddPreStopLifecycleHooks(workspace, podAdditions.Containers); err != nil {
		return nil, err
	}

	for _, initComponent := range initComponents {
		k8sContainer, err := convertContainerToK8s(initComponent, securityContext, pullPolicy, defaultResources)
		if err != nil {
			return nil, err
		}
		if err := handleMountSources(k8sContainer, initComponent.Container, workspace); err != nil {
			return nil, err
		}
		if overrides.NeedsContainerOverride(&initComponent) {
			patchedContainer, err := overrides.ApplyContainerOverrides(&initComponent, k8sContainer)
			if err != nil {
				return nil, err
			}
			k8sContainer = patchedContainer
		}
		podAdditions.InitContainers = append(podAdditions.InitContainers, *k8sContainer)
	}

	return podAdditions, nil
}
