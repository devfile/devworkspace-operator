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
// Package projects defines library functions for reconciling projects in a Devfile (i.e. cloning and maintaining state)
package projects

import (
	"fmt"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	devfileConstants "github.com/devfile/devworkspace-operator/pkg/library/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/devfile/devworkspace-operator/internal/images"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

const (
	projectClonerContainerName = "project-clone"
)

func GetProjectCloneInitContainer(workspaceWithConfig *common.DevWorkspaceWithConfig) (*corev1.Container, error) {

	workspace := workspaceWithConfig.Spec.Template
	if len(workspace.Projects) == 0 {
		return nil, nil
	}
	if workspace.Attributes.GetString(constants.ProjectCloneAttribute, nil) == constants.ProjectCloneDisable {
		return nil, nil
	}
	if !hasContainerComponents(&workspace) {
		// Avoid adding project-clone init container when DevWorkspace does not define any containers
		return nil, nil
	}

	cloneImage := images.GetProjectClonerImage()
	if cloneImage == "" {
		// Assume project clone is intentionally disabled if project clone image is not defined
		return nil, nil
	}
	memLimit, err := resource.ParseQuantity(constants.ProjectCloneMemoryLimit)
	if err != nil {
		return nil, fmt.Errorf("project clone container has invalid memory limit configured: %w", err)
	}
	memRequest, err := resource.ParseQuantity(constants.ProjectCloneMemoryRequest)
	if err != nil {
		return nil, fmt.Errorf("project clone container has invalid memory request configured: %w", err)
	}
	cpuLimit, err := resource.ParseQuantity(constants.ProjectCloneCPULimit)
	if err != nil {
		return nil, fmt.Errorf("project clone container has invalid CPU limit configured: %w", err)
	}
	cpuRequest, err := resource.ParseQuantity(constants.ProjectCloneCPURequest)
	if err != nil {
		return nil, fmt.Errorf("project clone container has invalid CPU request configured: %w", err)
	}

	return &corev1.Container{
		Name:  projectClonerContainerName,
		Image: cloneImage,
		Env: []corev1.EnvVar{
			// TODO: add proxy env
			{
				Name:  devfileConstants.ProjectsRootEnvVar,
				Value: constants.DefaultProjectsSourcesRoot,
			},
		},
		Resources: corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: memLimit,
				corev1.ResourceCPU:    cpuLimit,
			},
			Requests: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: memRequest,
				corev1.ResourceCPU:    cpuRequest,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      devfileConstants.ProjectsVolumeName,
				MountPath: constants.DefaultProjectsSourcesRoot,
			},
		},
		ImagePullPolicy: corev1.PullPolicy(workspaceWithConfig.Config.Workspace.ImagePullPolicy),
	}, nil
}

func hasContainerComponents(workspace *dw.DevWorkspaceTemplateSpec) bool {
	for _, component := range workspace.Components {
		if component.Container != nil {
			return true
		}
	}
	return false
}
