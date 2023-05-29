//
// Copyright (c) 2019-2023 Red Hat, Inc.
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
	"context"
	"fmt"

	"github.com/devfile/devworkspace-operator/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	devfileConstants "github.com/devfile/devworkspace-operator/pkg/library/constants"
	"github.com/devfile/devworkspace-operator/pkg/library/env"
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

func GetProjectCloneInitContainer(client client.Client, namespace string, workspace *dw.DevWorkspaceTemplateSpec, options Options, proxyConfig *controllerv1alpha1.Proxy) (*corev1.Container, error) {
	if len(workspace.Projects) == 0 {
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

	cloneEnv := []corev1.EnvVar{
		{
			Name:  devfileConstants.ProjectsRootEnvVar,
			Value: constants.DefaultProjectsSourcesRoot,
		},
	}
	cloneEnv = append(cloneEnv, env.GetProxyEnvVars(proxyConfig)...)
	cloneEnv = append(cloneEnv, options.Env...)

	resources, err := processResources(client, namespace, options.Resources)
	if err != nil {
		return nil, err
	}

	return &corev1.Container{
		Name:      projectClonerContainerName,
		Image:     cloneImage,
		Env:       cloneEnv,
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

// processResources checks that specified resources are valid (e.g. requests are less than limits) and supports
// un-setting resources that have default values by interpreting zero as "do not set"
func processResources(k8sClient client.Client, namespace string, resources *corev1.ResourceRequirements) (*corev1.ResourceRequirements, error) {
	result := resources.DeepCopy()

	if result.Limits.Memory().IsZero() {
		delete(result.Limits, corev1.ResourceMemory)
	}

	cpuLimit, hasCpuLimit := result.Limits[corev1.ResourceCPU]
	if !hasCpuLimit {
		result.Limits[corev1.ResourceCPU] = config.ProjectCloneContainerDefaultCpuLimit

		// Set empty CPU limits when possible:
		// 1. If there is no LimitRange in the namespace
		// 2. CPU limits is not overridden
		// See details at https://github.com/eclipse/che/issues/22198
		limitRanges := &corev1.LimitRangeList{}
		if err := k8sClient.List(context.TODO(), limitRanges, &client.ListOptions{Namespace: namespace}); err != nil {
			return nil, err
		} else if len(limitRanges.Items) == 0 {
			delete(result.Limits, corev1.ResourceCPU)
		}
	} else if cpuLimit.IsZero() {
		delete(result.Limits, corev1.ResourceCPU)
	}

	if result.Requests.Memory().IsZero() {
		delete(result.Requests, corev1.ResourceMemory)
	}
	if result.Requests.Cpu().IsZero() {
		delete(result.Requests, corev1.ResourceCPU)
	}

	memLimit, hasMemLimit := result.Limits[corev1.ResourceMemory]
	memRequest, hasMemRequest := result.Requests[corev1.ResourceMemory]
	if hasMemLimit && hasMemRequest && memRequest.Cmp(memLimit) > 0 {
		return result, fmt.Errorf("project clone memory request (%s) must be less than limit (%s)", memRequest.String(), memLimit.String())
	}

	cpuLimit, hasCPULimit := result.Limits[corev1.ResourceCPU]
	cpuRequest, hasCPURequest := result.Requests[corev1.ResourceCPU]
	if hasCPULimit && hasCPURequest && cpuRequest.Cmp(cpuLimit) > 0 {
		return result, fmt.Errorf("project clone CPU request (%s) must be less than limit (%s)", cpuRequest.String(), cpuLimit.String())
	}

	if len(result.Limits) == 0 {
		result.Limits = nil
	}
	if len(result.Requests) == 0 {
		result.Requests = nil
	}

	return result, nil
}

func hasContainerComponents(workspace *dw.DevWorkspaceTemplateSpec) bool {
	for _, component := range workspace.Components {
		if component.Container != nil {
			return true
		}
	}
	return false
}
