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
func GetKubeContainersFromDevfile(workspace *dw.DevWorkspaceTemplateSpec) (*v1alpha1.PodAdditions, error) {
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
		k8sContainer, err := convertContainerToK8s(component)
		if err != nil {
			return nil, err
		}
		handleMountSources(k8sContainer, component.Container)
		podAdditions.Containers = append(podAdditions.Containers, *k8sContainer)
	}

	for _, container := range initContainers {
		k8sContainer, err := convertContainerToK8s(container)
		if err != nil {
			return nil, err
		}
		handleMountSources(k8sContainer, container.Container)
		podAdditions.InitContainers = append(podAdditions.InitContainers, *k8sContainer)
	}

	return podAdditions, nil
}
