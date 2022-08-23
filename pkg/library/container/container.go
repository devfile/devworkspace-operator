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

// Package container contains library functions for converting DevWorkspace Container components to Kubernetes
// components
//
// TODO:
// - Devfile API spec is unclear on how mountSources should be handled -- mountPath is assumed to be /projects
//   and volume name is assumed to be "projects"
//   see issues:
//     - https://github.com/devfile/api/issues/290
//     - https://github.com/devfile/api/issues/291
package container

import (
	"fmt"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/library/flatten"
	"github.com/devfile/devworkspace-operator/pkg/library/lifecycle"
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
func GetKubeContainersFromDevfile(workspace *dw.DevWorkspaceTemplateSpec, config v1alpha1.OperatorConfiguration) (*v1alpha1.PodAdditions, error) {
	if !flatten.DevWorkspaceIsFlattened(workspace) {
		return nil, fmt.Errorf("devfile is not flattened")
	}
	podAdditions := &v1alpha1.PodAdditions{}

	initContainers, mainComponents, err := lifecycle.GetInitContainers(workspace.DevWorkspaceTemplateSpecContent)
	if err != nil {
		return nil, err
	}

	for _, component := range mainComponents {
		if component.Container == nil {
			continue
		}
		k8sContainer, err := convertContainerToK8s(component, config)
		if err != nil {
			return nil, err
		}
		handleMountSources(k8sContainer, component.Container, workspace.Projects)
		podAdditions.Containers = append(podAdditions.Containers, *k8sContainer)
	}

	if err := lifecycle.AddPostStartLifecycleHooks(workspace, podAdditions.Containers); err != nil {
		return nil, err
	}

	for _, container := range initContainers {
		k8sContainer, err := convertContainerToK8s(container, config)
		if err != nil {
			return nil, err
		}
		handleMountSources(k8sContainer, container.Container, workspace.Projects)
		podAdditions.InitContainers = append(podAdditions.InitContainers, *k8sContainer)
	}

	return podAdditions, nil
}
