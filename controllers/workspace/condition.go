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

	"github.com/devfile/devworkspace-operator/pkg/conditions"
)

var conditionOrder = []dw.DevWorkspaceConditionType{
	conditions.Started,
	conditions.DevWorkspaceResolved,
	conditions.StorageReady,
	dw.DevWorkspaceRoutingReady,
	dw.DevWorkspaceServiceAccountReady,
	conditions.PullSecretsReady,
	conditions.DeploymentReady,
	dw.DevWorkspaceReady,
}

// workspaceConditions is a description of last-observed workspace conditions.
type workspaceConditions struct {
	conditions map[dw.DevWorkspaceConditionType]dw.DevWorkspaceCondition
}

func (c *workspaceConditions) setConditionTrue(conditionType dw.DevWorkspaceConditionType, msg string) {
	if c.conditions == nil {
		c.conditions = map[dw.DevWorkspaceConditionType]dw.DevWorkspaceCondition{}
	}

	c.conditions[conditionType] = dw.DevWorkspaceCondition{
		Status:  corev1.ConditionTrue,
		Message: msg,
	}
}

func (c *workspaceConditions) setCondition(conditionType dw.DevWorkspaceConditionType, dwCondition dw.DevWorkspaceCondition) {
	if c.conditions == nil {
		c.conditions = map[dw.DevWorkspaceConditionType]dw.DevWorkspaceCondition{}
	}

	c.conditions[conditionType] = dwCondition
}

func (c *workspaceConditions) setConditionFalse(conditionType dw.DevWorkspaceConditionType, msg string) {
	if c.conditions == nil {
		c.conditions = map[dw.DevWorkspaceConditionType]dw.DevWorkspaceCondition{}
	}

	c.conditions[conditionType] = dw.DevWorkspaceCondition{
		Status:  corev1.ConditionFalse,
		Message: msg,
	}
}

// getFirstFalse checks current conditions in a set order (defined by conditionOrder) and returns the first
// condition with a 'false' status. Returns nil if there is no currently observed false condition
func (c *workspaceConditions) getFirstFalse() *dw.DevWorkspaceCondition {
	for _, cond := range conditionOrder {
		if condition, present := c.conditions[cond]; present && condition.Status == corev1.ConditionFalse {
			return &condition
		}
	}
	return nil
}

func (c *workspaceConditions) getLastTrue() *dw.DevWorkspaceCondition {
	var latestCondition *dw.DevWorkspaceCondition
	for _, cond := range conditionOrder {
		if condition, present := c.conditions[cond]; present && condition.Status == corev1.ConditionTrue {
			latestCondition = &condition
		}
	}
	return latestCondition
}

func getConditionIndexInOrder(condType dw.DevWorkspaceConditionType) int {
	for i, cond := range conditionOrder {
		if cond == condType {
			return i
		}
	}
	return -1
}
