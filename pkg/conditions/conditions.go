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

package conditions

import dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"

const (
	Started              dw.DevWorkspaceConditionType = "Started"
	PullSecretsReady     dw.DevWorkspaceConditionType = "PullSecretsReady"
	DevWorkspaceResolved dw.DevWorkspaceConditionType = "DevWorkspaceResolved"
	StorageReady         dw.DevWorkspaceConditionType = "StorageReady"
	DeploymentReady      dw.DevWorkspaceConditionType = "DeploymentReady"
	DevWorkspaceWarning  dw.DevWorkspaceConditionType = "DevWorkspaceWarning"
)

func GetConditionByType(conditions []dw.DevWorkspaceCondition, t dw.DevWorkspaceConditionType) *dw.DevWorkspaceCondition {
	for _, condition := range conditions {
		if condition.Type == t {
			return &condition
		}
	}
	return nil
}
