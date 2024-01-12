//
// Copyright (c) 2019-2024 Red Hat, Inc.
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
	conditions.KubeComponentsReady,
	conditions.DeploymentReady,
	dw.DevWorkspaceReady,
}

// workspaceConditions is a description of last-observed workspace conditions.
type workspaceConditions struct {
	conditions        map[dw.DevWorkspaceConditionType]dw.DevWorkspaceCondition
	warningConditions []dw.DevWorkspaceCondition
}

func workspaceConditionsFromClusterObject(clusterConditions []dw.DevWorkspaceCondition) *workspaceConditions {
	wkspConditions := &workspaceConditions{
		conditions: map[dw.DevWorkspaceConditionType]dw.DevWorkspaceCondition{},
	}
	for _, condition := range clusterConditions {
		if condition.Type == conditions.DevWorkspaceWarning {
			wkspConditions.warningConditions = append(wkspConditions.warningConditions, condition)
		} else {
			wkspConditions.conditions[condition.Type] = condition
		}
	}
	return wkspConditions
}

func (c *workspaceConditions) setConditionTrue(conditionType dw.DevWorkspaceConditionType, msg string) {
	c.setConditionTrueWithReason(conditionType, msg, "")
}

func (c *workspaceConditions) setConditionTrueWithReason(conditionType dw.DevWorkspaceConditionType, msg string, reason string) {
	if c.conditions == nil {
		c.conditions = map[dw.DevWorkspaceConditionType]dw.DevWorkspaceCondition{}
	}

	c.conditions[conditionType] = dw.DevWorkspaceCondition{
		Status:  corev1.ConditionTrue,
		Message: msg,
		Reason:  reason,
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

func (c *workspaceConditions) addWarning(msg string) {
	c.addWarningWithReason(msg, "")
}

func (c *workspaceConditions) addWarningWithReason(msg, reason string) {
	c.warningConditions = append(c.warningConditions, dw.DevWorkspaceCondition{
		Status:  corev1.ConditionTrue,
		Message: msg,
		Reason:  reason,
	})
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
