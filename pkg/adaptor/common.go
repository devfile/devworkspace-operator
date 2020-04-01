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
	"fmt"
	"github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	"github.com/che-incubator/che-workspace-operator/pkg/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func SortComponentsByType(components []v1alpha1.ComponentSpec) (dockerimage, plugin []v1alpha1.ComponentSpec, err error) {
	for _, component := range components {
		switch component.Type {
		case v1alpha1.Dockerimage:
			dockerimage = append(dockerimage, component)
		case v1alpha1.CheEditor, v1alpha1.ChePlugin:
			plugin = append(plugin, component)
		default:
			return nil, nil, fmt.Errorf("unsupported component type encountered: %s", component.Type)
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
