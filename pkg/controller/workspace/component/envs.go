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

	. "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/model"
)

func commonEnvironmentVariables(wkspProps WorkspaceProperties) []corev1.EnvVar {
	return []corev1.EnvVar{
		corev1.EnvVar{
			Name: "CHE_MACHINE_TOKEN",
		},
		corev1.EnvVar{
			Name:  "CHE_PROJECTS_ROOT",
			Value: "/projects",
		},
		corev1.EnvVar{
			Name:  "CHE_API",
			Value: wkspProps.CheApiExternal,
		},
		corev1.EnvVar{
			Name:  "CHE_API_INTERNAL",
			Value: DefaultApiEndpoint,
		},
		corev1.EnvVar{
			Name:  "CHE_API_EXTERNAL",
			Value: wkspProps.CheApiExternal,
		},
		corev1.EnvVar{
			Name:  "CHE_WORKSPACE_NAME",
			Value: wkspProps.WorkspaceName,
		},
		corev1.EnvVar{
			Name:  "CHE_WORKSPACE_ID",
			Value: wkspProps.WorkspaceId,
		},
		corev1.EnvVar{
			Name:  "CHE_AUTH_ENABLED",
			Value: AuthEnabled,
		},
		corev1.EnvVar{
			Name:  "CHE_WORKSPACE_NAMESPACE",
			Value: wkspProps.Namespace,
		},
	}
}
