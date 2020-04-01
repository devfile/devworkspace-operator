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

package workspace

import (
	"context"
	"fmt"
	"github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	"github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/provision"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclock "k8s.io/apimachinery/pkg/util/clock"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sort"
	"strings"
)

// clock is used to set status condition timestamps.
// This variable makes it easier to test conditions.
var clock kubeclock.Clock = &kubeclock.RealClock{}

func (r *ReconcileWorkspace) updateWorkspaceStatus(workspace *v1alpha1.Workspace, clusterAPI provision.ClusterAPI, status *currentStatus, reconcileResult reconcile.Result, reconcileError error) (reconcile.Result, error) {
	workspace.Status.Phase = status.Phase
	currTransitionTime := metav1.Time{Time: clock.Now()}
	for _, conditionType := range status.Conditions {
		conditionExists := false
		for idx, condition := range workspace.Status.Condition {
			if condition.Type == conditionType && condition.LastTransitionTime.Before(&currTransitionTime) {
				workspace.Status.Condition[idx].LastTransitionTime = currTransitionTime
				workspace.Status.Condition[idx].Status = corev1.ConditionTrue
				conditionExists = true
				break
			}
		}
		if !conditionExists {
			workspace.Status.Condition = append(workspace.Status.Condition, v1alpha1.WorkspaceCondition{
				Type:               conditionType,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: currTransitionTime,
			})
		}
	}
	for idx, condition := range workspace.Status.Condition {
		if condition.LastTransitionTime.Before(&currTransitionTime) {
			workspace.Status.Condition[idx].LastTransitionTime = currTransitionTime
			workspace.Status.Condition[idx].Status = corev1.ConditionUnknown
		}
	}
	sort.SliceStable(workspace.Status.Condition, func(i, j int) bool {
		return strings.Compare(string(workspace.Status.Condition[i].Type), string(workspace.Status.Condition[j].Type)) > 0
	})

	err := r.client.Status().Update(context.TODO(), workspace)
	if err != nil {
		clusterAPI.Logger.Info(fmt.Sprintf("Error updating workspace status: %s", err))
		if reconcileError == nil {
			reconcileError = err
		}
	}
	return reconcileResult, err
}
