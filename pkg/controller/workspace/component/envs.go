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

package component

import (
	corev1 "k8s.io/api/core/v1"

	. "github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/model"
)

func commonEnvironmentVariables(wkspCtx WorkspaceContext) []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name: "CHE_MACHINE_TOKEN",
		},
		{
			Name:  "CHE_PROJECTS_ROOT",
			Value: DefaultProjectsSourcesRoot,
		},
		{
			Name:  "CHE_API",
			Value: wkspCtx.CheApiExternal,
		},
		{
			Name:  "CHE_API_INTERNAL",
			Value: DefaultApiEndpoint,
		},
		{
			Name:  "CHE_API_EXTERNAL",
			Value: wkspCtx.CheApiExternal,
		},
		{
			Name:  "CHE_WORKSPACE_NAME",
			Value: wkspCtx.WorkspaceName,
		},
		{
			Name:  "CHE_WORKSPACE_ID",
			Value: wkspCtx.WorkspaceId,
		},
		{
			Name:  "CHE_AUTH_ENABLED",
			Value: AuthEnabled,
		},
		{
			Name:  "CHE_WORKSPACE_NAMESPACE",
			Value: wkspCtx.Namespace,
		},
	}
}
