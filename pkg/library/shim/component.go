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

// package shim contains functions for generating metadata needed by Che-Theia for correct
// representation of workspaces. These functions serve to enable compatibility between devfile 2.0
// workspaces and Che-Theia until Theia gets devfile 2.0 support.
package shim

import (
	"github.com/devfile/devworkspace-operator/pkg/constants"

	devworkspace "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	corev1 "k8s.io/api/core/v1"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
)

func FillDefaultEnvVars(podAdditions *v1alpha1.PodAdditions, workspace devworkspace.DevWorkspace) {
	for idx, mainContainer := range podAdditions.Containers {
		podAdditions.Containers[idx].Env = append(mainContainer.Env, defaultEnvVars(mainContainer.Name, workspace)...)
	}

	for idx, mainContainer := range podAdditions.InitContainers {
		podAdditions.InitContainers[idx].Env = append(mainContainer.Env, defaultEnvVars(mainContainer.Name, workspace)...)
	}
}

func defaultEnvVars(containerName string, workspace devworkspace.DevWorkspace) []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "CHE_MACHINE_NAME",
			Value: containerName,
		},
		{
			Name: "CHE_MACHINE_TOKEN",
		},
		{
			Name:  "CHE_PROJECTS_ROOT",
			Value: constants.DefaultProjectsSourcesRoot,
		},
		{
			Name:  "CHE_API",
			Value: constants.DefaultApiEndpoint,
		},
		{
			Name:  "CHE_API_INTERNAL",
			Value: constants.DefaultApiEndpoint,
		},
		{
			Name:  "CHE_API_EXTERNAL",
			Value: constants.DefaultApiEndpoint,
		},
		{
			Name:  "CHE_WORKSPACE_NAME",
			Value: workspace.Name,
		},
		{
			Name:  "CHE_WORKSPACE_ID",
			Value: workspace.Status.WorkspaceId,
		},
		{
			Name:  "CHE_AUTH_ENABLED",
			Value: constants.AuthEnabled,
		},
		{
			Name:  "CHE_WORKSPACE_NAMESPACE",
			Value: workspace.Namespace,
		},
	}
}
