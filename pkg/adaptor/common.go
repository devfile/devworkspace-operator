//
// Copyright (c) 2019-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package adaptor

import (
	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha1"

	"github.com/devfile/devworkspace-operator/pkg/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func SortComponentsByType(components []devworkspace.Component) (dockerimages []devworkspace.Component, plugins []devworkspace.Component, err error) {
	for _, component := range components {
		err := component.Visit(devworkspace.ComponentVisitor{
			Plugin: func(plugin *devworkspace.PluginComponent) error {
				plugins = append(plugins, component)
				return nil
			},
			Container: func(container *devworkspace.ContainerComponent) error {
				dockerimages = append(dockerimages, component)
				return nil
			},
		})
		if err != nil {
			return nil, nil, err
		}
	}
	return
}

func adaptResourcesFromString(memLimit string) (corev1.ResourceRequirements, error) {
	if memLimit == "" {
		memLimit = config.SidecarDefaultMemoryLimit
	}
	memLimitQuantity, err := resource.ParseQuantity(memLimit)
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}
	resources := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: memLimitQuantity,
		},
		Requests: nil,
	}

	return resources, nil
}

func GetProjectSourcesVolumeMount(workspaceId string) corev1.VolumeMount {
	volumeName := config.ControllerCfg.GetWorkspacePVCName()

	projectsVolumeMount := corev1.VolumeMount{
		Name:      volumeName,
		MountPath: config.DefaultProjectsSourcesRoot,
		SubPath:   workspaceId + config.DefaultProjectsSourcesRoot,
	}

	return projectsVolumeMount
}
