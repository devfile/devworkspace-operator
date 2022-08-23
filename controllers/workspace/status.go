//
// Copyright (c) 2019-2022 Red Hat, Inc.
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
	"context"
	"fmt"
	"net/url"
	"sort"
	"time"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclock "k8s.io/apimachinery/pkg/util/clock"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/workspace/metrics"
	"github.com/devfile/devworkspace-operator/pkg/conditions"
)

const (
	// devworkspacePhaseTerminating represents a DevWorkspace that has been deleted but is waiting on a finalizer.
	// TODO: Should be moved to devfile/api side.
	devworkspacePhaseTerminating dw.DevWorkspacePhase = "Terminating"

	// devworkspacePhaseFailing represents a DevWorkspace that has encountered an unrecoverable error and is in
	// the process of stopping.
	devworkspacePhaseFailing dw.DevWorkspacePhase = "Failing"

	// warningPresentInfoMessage is the info message printed
	warningPresentInfoMessage string = "[warnings present]"
)

type currentStatus struct {
	workspaceConditions
	// Current workspace phase
	phase dw.DevWorkspacePhase
}

// clock is used to set status condition timestamps.
// This variable makes it easier to test conditions.
var clock kubeclock.Clock = &kubeclock.RealClock{}

// updateWorkspaceStatus updates the current workspace's status field with conditions and phase from the passed in status.
// Parameters for result and error are returned unmodified, unless error is nil and another error is encountered while
// updating the status.
func (r *DevWorkspaceReconciler) updateWorkspaceStatus(workspace *dw.DevWorkspace, logger logr.Logger, status *currentStatus, reconcileResult reconcile.Result, reconcileError error) (reconcile.Result, error) {
	syncConditions(&workspace.Status, status)
	oldPhase := workspace.Status.Phase
	workspace.Status.Phase = status.phase

	infoMessage := getInfoMessage(workspace, status)
	if warn := conditions.GetConditionByType(workspace.Status.Conditions, conditions.DevWorkspaceWarning); warn != nil && warn.Status == corev1.ConditionTrue {
		infoMessage = fmt.Sprintf("%s %s", warningPresentInfoMessage, infoMessage)
	}
	if workspace.Status.Message != infoMessage {
		workspace.Status.Message = infoMessage
	}

	err := r.Status().Update(context.TODO(), workspace)
	if err != nil {
		if k8sErrors.IsConflict(err) {
			logger.Info("Failed to update workspace status due to conflict; retrying")
		} else {
			logger.Info(fmt.Sprintf("Error updating workspace status: %s", err))
			if reconcileError == nil {
				reconcileError = err
			}
		}
	} else {
		updateMetricsForPhase(workspace, oldPhase, status.phase, logger)
	}

	return reconcileResult, reconcileError
}

func syncConditions(workspaceStatus *dw.DevWorkspaceStatus, currentStatus *currentStatus) {
	currTransitionTime := metav1.Time{Time: clock.Now()}

	// Set of conditions already set on the workspace
	existingConditions := map[dw.DevWorkspaceConditionType]bool{}
	for idx, workspaceCondition := range workspaceStatus.Conditions {
		existingConditions[workspaceCondition.Type] = true

		currCondition, ok := currentStatus.conditions[workspaceCondition.Type]
		if !ok {
			// Didn't observe this condition this time; set status to unknown
			if workspaceCondition.Status != corev1.ConditionUnknown {
				workspaceStatus.Conditions[idx].LastTransitionTime = currTransitionTime
				workspaceStatus.Conditions[idx].Status = corev1.ConditionUnknown
				workspaceStatus.Conditions[idx].Message = ""
				workspaceStatus.Conditions[idx].Reason = ""
			}
			continue
		}

		// Update condition if needed
		if workspaceCondition.Status != currCondition.Status || workspaceCondition.Message != currCondition.Message || workspaceCondition.Reason != currCondition.Reason {
			workspaceStatus.Conditions[idx].LastTransitionTime = currTransitionTime
			workspaceStatus.Conditions[idx].Status = currCondition.Status
			workspaceStatus.Conditions[idx].Message = currCondition.Message
			workspaceStatus.Conditions[idx].Reason = currCondition.Reason
		}
	}

	// Check for conditions we need to add
	for condType, cond := range currentStatus.conditions {
		if existingConditions[condType] {
			// Condition is already present and was updated (if necessary) above
			continue
		}
		workspaceStatus.Conditions = append(workspaceStatus.Conditions, dw.DevWorkspaceCondition{
			LastTransitionTime: currTransitionTime,
			Type:               condType,
			Status:             cond.Status,
			Message:            cond.Message,
			Reason:             cond.Reason,
		})
	}

	// Sort conditions to avoid unnecessary updates
	sort.SliceStable(workspaceStatus.Conditions, func(i, j int) bool {
		return getConditionIndexInOrder(workspaceStatus.Conditions[i].Type) < getConditionIndexInOrder(workspaceStatus.Conditions[j].Type)
	})
}

func syncWorkspaceMainURL(workspace *dw.DevWorkspace, exposedEndpoints map[string]v1alpha1.ExposedEndpointList, clusterAPI sync.ClusterAPI) (ok bool, err error) {
	mainUrl := getMainUrl(exposedEndpoints)

	if workspace.Status.MainUrl == mainUrl {
		return true, nil
	}
	workspace.Status.MainUrl = mainUrl
	err = clusterAPI.Client.Status().Update(context.TODO(), workspace)
	return false, err
}

func checkServerStatus(workspace *dw.DevWorkspace) (ok bool, err error) {
	mainUrl := workspace.Status.MainUrl
	if mainUrl == "" {
		// Support DevWorkspaces that do not specify an mainUrl
		return true, nil
	}
	healthz, err := url.Parse(mainUrl)
	if err != nil {
		return false, err
	}
	healthz.Path = healthz.Path + "healthz"

	resp, err := healthCheckHttpClient.Get(healthz.String())
	if err != nil {
		return false, err
	}
	if (resp.StatusCode / 100) == 4 {
		// Assume endpoint is unimplemented and/or * is covered with authentication.
		return true, nil
	}
	ok = (resp.StatusCode / 100) == 2
	return ok, nil
}

func getMainUrl(exposedEndpoints map[string]v1alpha1.ExposedEndpointList) string {
	for _, endpoints := range exposedEndpoints {
		for _, endpoint := range endpoints {
			if endpoint.Attributes.GetString(string(v1alpha1.TypeEndpointAttribute), nil) == string(v1alpha1.MainEndpointType) {
				return endpoint.Url
			}
		}
	}
	return ""
}

func getInfoMessage(workspace *dw.DevWorkspace, status *currentStatus) string {
	// Check for errors and failure
	if cond, ok := status.conditions[dw.DevWorkspaceError]; ok {
		return cond.Message
	}
	if cond, ok := status.conditions[dw.DevWorkspaceFailedStart]; ok {
		return cond.Message
	}
	switch workspace.Status.Phase {
	case dw.DevWorkspaceStatusRunning:
		if workspace.Status.MainUrl == "" {
			return "Workspace is running"
		}
		return workspace.Status.MainUrl
	case dw.DevWorkspaceStatusStopped, dw.DevWorkspaceStatusStopping:
		return string(workspace.Status.Phase)
	}

	latestCondition := status.getFirstFalse()
	if latestCondition != nil {
		return latestCondition.Message
	}

	latestTrueCondition := status.getLastTrue()
	if latestTrueCondition != nil {
		return latestTrueCondition.Message
	}

	// No conditions are set but workspace is not running; unclear what value should be set.
	return ""
}

// updateMetricsForPhase increments DevWorkspace startup metrics based on phase transitions in a DevWorkspace. It avoids
// incrementing the underlying metrics where possible (e.g. reconciling an already running workspace) by only incrementing
// counters when the new phase is different from the current on in the DevWorkspace.
func updateMetricsForPhase(workspace *dw.DevWorkspace, oldPhase, newPhase dw.DevWorkspacePhase, logger logr.Logger) {
	if oldPhase == newPhase {
		return
	}
	switch newPhase {
	case dw.DevWorkspaceStatusRunning:
		metrics.WorkspaceRunning(workspace, logger)
	case dw.DevWorkspaceStatusFailed:
		metrics.WorkspaceFailed(workspace, logger)
	}
}

// checkForStartTimeout checks if the provided workspace has not progressed for longer than the configured
// startup timeout. This is determined by checking to see if the last condition transition time is more
// than [timeout] duration ago. Workspaces that are not in the "Starting" phase cannot timeout. Returns
// an error with message when timeout is reached.
func checkForStartTimeout(workspace *dw.DevWorkspace, config controllerv1alpha1.OperatorConfiguration) error {
	if workspace.Status.Phase != dw.DevWorkspaceStatusStarting {
		return nil
	}
	timeout, err := time.ParseDuration(config.Workspace.ProgressTimeout)
	if err != nil {
		return fmt.Errorf("invalid duration specified for timeout: %w", err)
	}
	currTime := clock.Now()
	lastUpdateTime := time.Time{}
	for _, condition := range workspace.Status.Conditions {
		if condition.LastTransitionTime.Time.After(lastUpdateTime) {
			lastUpdateTime = condition.LastTransitionTime.Time
		}
	}
	if !lastUpdateTime.IsZero() && lastUpdateTime.Add(timeout).Before(currTime) {
		return fmt.Errorf("devworkspace failed to progress past phase '%s' for longer than timeout (%s)",
			workspace.Status.Phase, config.Workspace.ProgressTimeout)
	}
	return nil
}

// checkForFailingTimeout checks that the current workspace has not been in the "Failing" state for longer than the
// configured progress timeout. If the workspace is not in the Failing state or does not have a DevWorkspaceFailed
// condition set, returns false. Otherwise, returns true if the workspace has timed out. Returns an error if
// timeout is configured with an unparsable duration.
func checkForFailingTimeout(workspaceWithConfig *common.DevWorkspaceWithConfig) (isTimedOut bool, err error) {
	if workspaceWithConfig.Status.Phase != devworkspacePhaseFailing {
		return false, nil
	}
	timeout, err := time.ParseDuration(workspaceWithConfig.Config.Workspace.ProgressTimeout)
	if err != nil {
		return false, fmt.Errorf("invalid duration specified for timeout: %w", err)
	}
	currTime := clock.Now()
	failedTime := time.Time{}
	for _, condition := range workspaceWithConfig.Status.Conditions {
		if condition.Type == dw.DevWorkspaceFailedStart {
			failedTime = condition.LastTransitionTime.Time
		}
	}
	if !failedTime.IsZero() && failedTime.Add(timeout).Before(currTime) {
		return true, nil
	}
	return false, nil
}
