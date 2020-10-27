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

package env

import (
	"github.com/devfile/devworkspace-operator/pkg/config"
	corev1 "k8s.io/api/core/v1"
)

func CommonEnvironmentVariables(workspaceName, workspaceId, namespace, creator string) []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name: "CHE_MACHINE_TOKEN",
		},
		{
			Name:  "CHE_PROJECTS_ROOT",
			Value: config.DefaultProjectsSourcesRoot,
		},
		{
			Name:  "CHE_API",
			Value: config.DefaultApiEndpoint,
		},
		{
			Name:  "CHE_API_INTERNAL",
			Value: config.DefaultApiEndpoint,
		},
		{
			Name:  "CHE_API_EXTERNAL",
			Value: config.DefaultApiEndpoint,
		},
		{
			Name:  "CHE_WORKSPACE_NAME",
			Value: workspaceName,
		},
		{
			Name:  "CHE_WORKSPACE_ID",
			Value: workspaceId,
		},
		{
			Name:  "CHE_AUTH_ENABLED",
			Value: config.AuthEnabled,
		},
		{
			Name:  "CHE_WORKSPACE_NAMESPACE",
			Value: namespace,
		},
		{
			Name:  "USE_BEARER_TOKEN",
			Value: config.ControllerCfg.GetWebhooksEnabled(),
		},
		{
			Name:  "DEVWORKSPACE_CREATOR",
			Value: creator,
		},
		{
			Name:  "DEVWORKSPACE_IDLE_TIMEOUT",
			Value: config.ControllerCfg.GetWorkspaceIdleTimeout(),
		},
	}
}
