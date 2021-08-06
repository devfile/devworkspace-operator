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

package workspace

import (
	v1 "k8s.io/api/core/v1"

	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

func CommonEnvironmentVariables(workspaceName, workspaceId, namespace, creator string) []v1.EnvVar {
	return []v1.EnvVar{
		{
			Name:  constants.DevWorkspaceNamespace,
			Value: namespace,
		},
		{
			Name:  constants.DevWorkspaceName,
			Value: workspaceName,
		},
		{
			Name:  constants.DevWorkspaceId,
			Value: workspaceId,
		},
		{
			Name:  constants.DevWorkspaceCreator,
			Value: creator,
		},
		{
			Name:  constants.DevWorkspaceIdleTimeout,
			Value: config.ControllerCfg.GetWorkspaceIdleTimeout(),
		},
	}
}
