//
// Copyright (c) 2019-2021 Red Hat, Inc.
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
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"sort"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	"github.com/devfile/devworkspace-operator/pkg/timing"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclock "k8s.io/apimachinery/pkg/util/clock"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
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

// healthHttpClient is supposed to be used for performing health checks of workspace endpoints
var healthHttpClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
}

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
		logger.Info(fmt.Sprintf("Error updating workspace status: %s", err))
		if reconcileError == nil {
			reconcileError = err
		}
	} else {
		metrics.UpdateMetricsForPhase(workspace, oldPhase, status.phase, logger)
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

	resp, err := healthHttpClient.Get(healthz.String())
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

func (r *DevWorkspaceReconciler) syncStartedAtToCluster(
	ctx context.Context, workspace *dw.DevWorkspace, reqLogger logr.Logger) {

	if workspace.Annotations == nil {
		workspace.Annotations = map[string]string{}
	}

	if _, hasStartedAtAnnotation := workspace.Annotations[constants.DevWorkspaceStartedAtAnnotation]; hasStartedAtAnnotation {
		return
	}

	workspace.Annotations[constants.DevWorkspaceStartedAtAnnotation] = timing.CurrentTime()
	if err := r.Update(ctx, workspace); err != nil {
		if k8sErrors.IsConflict(err) {
			reqLogger.Info("Got conflict when trying to apply started-at annotations to workspace")
		} else {
			reqLogger.Error(err, "Error trying to apply started-at annotation to devworkspace")
		}
	}
}

func (r *DevWorkspaceReconciler) removeStartedAtFromCluster(
	ctx context.Context, workspace *dw.DevWorkspace, reqLogger logr.Logger) {
	if workspace.Annotations == nil {
		workspace.Annotations = map[string]string{}
	}
	delete(workspace.Annotations, constants.DevWorkspaceStartedAtAnnotation)
	if err := r.Update(ctx, workspace); err != nil {
		if k8sErrors.IsConflict(err) {
			reqLogger.Info("Got conflict when trying to apply timing annotations to workspace")
		} else {
			reqLogger.Error(err, "Error trying to apply timing annotations to devworkspace")
		}
	}
}
