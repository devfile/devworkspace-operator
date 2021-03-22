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

package controllers

import (
	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	corev1 "k8s.io/api/core/v1"
)

const (
	PullSecretsReady     dw.WorkspaceConditionType = "PullSecretsReady"
	DevWorkspaceResolved dw.WorkspaceConditionType = "DevWorkspaceResolved"
	StorageReady         dw.WorkspaceConditionType = "StorageReady"
	DeploymentReady      dw.WorkspaceConditionType = "DeploymentReady"
)

var conditionOrder = []dw.WorkspaceConditionType{
	DevWorkspaceResolved,
	StorageReady,
	dw.WorkspaceRoutingReady,
	dw.WorkspaceServiceAccountReady,
	PullSecretsReady,
	DeploymentReady,
	dw.WorkspaceReady,
}

// workspaceConditions is a description of last-observed workspace conditions.
type workspaceConditions struct {
	conditions map[dw.WorkspaceConditionType]dw.WorkspaceCondition
}

func (c *workspaceConditions) setConditionTrue(conditionType dw.WorkspaceConditionType, msg string) {
	c.conditions[conditionType] = dw.WorkspaceCondition{
		Status:  corev1.ConditionTrue,
		Message: msg,
	}
}

func (c *workspaceConditions) setConditionFalse(conditionType dw.WorkspaceConditionType, msg string) {
	c.conditions[conditionType] = dw.WorkspaceCondition{
		Status:  corev1.ConditionFalse,
		Message: msg,
	}
}

// getFirstFalse checks current conditions in a set order (defined by conditionOrder) and returns the first
// condition with a 'false' status. Returns nil if there is no currently observed false condition
func (c *workspaceConditions) getFirstFalse() *dw.WorkspaceCondition {
	for _, cond := range conditionOrder {
		if condition, present := c.conditions[cond]; present && condition.Status == corev1.ConditionFalse {
			return &condition
		}
	}
	return nil
}
