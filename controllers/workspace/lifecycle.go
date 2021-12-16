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

package controllers

import (
	"context"
	"fmt"
	"time"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/workspace/metrics"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/conditions"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	wsprovision "github.com/devfile/devworkspace-operator/pkg/provision/workspace"
	"github.com/devfile/devworkspace-operator/pkg/timing"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// checkWorkspaceShouldBeStarted checks whether the controller should continue reconciling the provided DevWorkspace Resource, signaling that
// reconcile should stop if the workspace is stopped, failed, or needs initialization. Returns ok = true if reconcile can continue,
// otherwise returns false and the reconcile.Result and error that should be returned from the reconcile loop.
func (r *DevWorkspaceReconciler) checkWorkspaceShouldBeStarted(workspace *dw.DevWorkspace, ctx context.Context, reqLogger logr.Logger) (ok bool, res reconcile.Result, err error) {
	// Check if the DevWorkspaceRouting instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	if workspace.GetDeletionTimestamp() != nil {
		reqLogger.Info("Finalizing DevWorkspace")
		res, err := r.finalize(ctx, reqLogger, workspace)
		return false, res, err
	}

	// Ensure workspaceID is set.
	if workspace.Status.DevWorkspaceId == "" {
		workspaceId, err := getWorkspaceId(workspace)
		if err != nil {
			return false, reconcile.Result{}, err
		}
		workspace.Status.DevWorkspaceId = workspaceId
		err = r.Status().Update(ctx, workspace)
		return false, reconcile.Result{Requeue: true}, err
	}

	// Stop failed workspaces
	if workspace.Status.Phase == devworkspacePhaseFailing && workspace.Spec.Started {
		// If debug annotation is present, leave the deployment in place to let users
		// view logs.
		if workspace.Annotations[constants.DevWorkspaceDebugStartAnnotation] == "true" {
			if isTimeout, err := checkForFailingTimeout(workspace); err != nil {
				return false, reconcile.Result{}, err
			} else if !isTimeout {
				return false, reconcile.Result{}, nil
			}
		}

		patch := []byte(`{"spec":{"started": false}}`)
		err := r.Client.Patch(context.Background(), workspace, client.RawPatch(types.MergePatchType, patch))
		if err != nil {
			return false, reconcile.Result{}, err
		}

		// Requeue reconcile to stop workspace
		return false, reconcile.Result{Requeue: true}, nil
	}

	// Handle stopped workspaces
	if !workspace.Spec.Started {
		timing.ClearAnnotations(workspace)
		r.removeStartedAtFromCluster(ctx, workspace, reqLogger)
		r.syncTimingToCluster(ctx, workspace, map[string]string{}, reqLogger)
		res, err := r.stopWorkspace(workspace, reqLogger)
		return false, res, err
	}

	// If this is the first reconcile for a starting workspace, mark it as starting now. This is done outside the regular
	// updateWorkspaceStatus function to ensure it gets set immediately
	if workspace.Status.Phase != dw.DevWorkspaceStatusStarting && workspace.Status.Phase != dw.DevWorkspaceStatusRunning {
		// Set 'Started' condition as early as possible to get accurate timing metrics
		workspace.Status.Phase = dw.DevWorkspaceStatusStarting
		workspace.Status.Message = "Initializing DevWorkspace"
		workspace.Status.Conditions = []dw.DevWorkspaceCondition{
			{
				Type:               conditions.Started,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Time{Time: clock.Now()},
				Message:            "DevWorkspace is starting",
			},
		}
		err = r.Status().Update(ctx, workspace)
		if err == nil {
			metrics.WorkspaceStarted(workspace, reqLogger)
		}
		return false, reconcile.Result{}, err
	}
	return true, reconcile.Result{}, nil
}

func (r *DevWorkspaceReconciler) stopWorkspace(workspace *dw.DevWorkspace, logger logr.Logger) (reconcile.Result, error) {
	status := currentStatus{phase: dw.DevWorkspaceStatusStopping}
	if workspace.Status.Phase == devworkspacePhaseFailing || workspace.Status.Phase == dw.DevWorkspaceStatusFailed {
		status.phase = workspace.Status.Phase
		failedCondition := conditions.GetConditionByType(workspace.Status.Conditions, dw.DevWorkspaceFailedStart)
		if failedCondition != nil {
			status.setCondition(dw.DevWorkspaceFailedStart, *failedCondition)
		}
	}

	stopped, err := r.doStop(workspace, logger)
	if err != nil {
		return reconcile.Result{}, err
	}

	if stopped {
		switch status.phase {
		case devworkspacePhaseFailing, dw.DevWorkspaceStatusFailed:
			status.phase = dw.DevWorkspaceStatusFailed
			status.setConditionFalse(conditions.Started, "Workspace stopped due to error")
		default:
			status.phase = dw.DevWorkspaceStatusStopped
			status.setConditionFalse(conditions.Started, "Workspace is stopped")
		}
	}
	return r.updateWorkspaceStatus(workspace, logger, &status, reconcile.Result{}, nil)
}

func (r *DevWorkspaceReconciler) doStop(workspace *dw.DevWorkspace, logger logr.Logger) (stopped bool, err error) {
	workspaceDeployment := &appsv1.Deployment{}
	namespaceName := types.NamespacedName{
		Name:      common.DeploymentName(workspace.Status.DevWorkspaceId),
		Namespace: workspace.Namespace,
	}
	err = r.Get(context.TODO(), namespaceName, workspaceDeployment)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}

	// Update DevWorkspaceRouting to have `devworkspace-started` annotation "false"
	routing := &v1alpha1.DevWorkspaceRouting{}
	routingRef := types.NamespacedName{
		Name:      common.DevWorkspaceRoutingName(workspace.Status.DevWorkspaceId),
		Namespace: workspace.Namespace,
	}
	err = r.Get(context.TODO(), routingRef, routing)
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			return false, err
		}
	} else if routing.Annotations != nil && routing.Annotations[constants.DevWorkspaceStartedStatusAnnotation] != "false" {
		routing.Annotations[constants.DevWorkspaceStartedStatusAnnotation] = "false"
		err := r.Update(context.TODO(), routing)
		if err != nil {
			if k8sErrors.IsConflict(err) {
				return false, nil
			}
			return false, err
		}
	}

	replicas := workspaceDeployment.Spec.Replicas
	if replicas == nil || *replicas > 0 {
		logger.Info("Stopping workspace")
		err = wsprovision.ScaleDeploymentToZero(workspace, r.Client)
		if err != nil && !k8sErrors.IsConflict(err) {
			return false, err
		}
		return false, nil
	}

	if workspaceDeployment.Status.Replicas == 0 {
		return true, nil
	}
	return false, nil
}

// failWorkspace marks a workspace as failed by setting relevant fields in the status struct.
// These changes are not synced to cluster immediately, and are intended to be synced to the cluster via a deferred function
// in the main reconcile loop. If needed, changes can be flushed to the cluster immediately via `updateWorkspaceStatus()`
func (r *DevWorkspaceReconciler) failWorkspace(workspace *dw.DevWorkspace, msg string, reason metrics.FailureReason, logger logr.Logger, status *currentStatus) (reconcile.Result, error) {
	logger.Info("DevWorkspace failed to start: " + msg)
	status.phase = devworkspacePhaseFailing
	status.setConditionTrueWithReason(dw.DevWorkspaceFailedStart, msg, string(reason))
	if workspace.Spec.Started {
		return reconcile.Result{Requeue: true}, nil
	}
	return reconcile.Result{}, nil
}

// checkForStartTimeout checks if the provided workspace has not progressed for longer than the configured
// startup timeout. This is determined by checking to see if the last condition transition time is more
// than [timeout] duration ago. Workspaces that are not in the "Starting" phase cannot timeout. Returns
// an error with message when timeout is reached.
func checkForStartTimeout(workspace *dw.DevWorkspace) error {
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
func checkForFailingTimeout(workspace *dw.DevWorkspace) (isTimedOut bool, err error) {
	if workspace.Status.Phase != devworkspacePhaseFailing {
		return false, nil
	}
	timeout, err := time.ParseDuration(config.Workspace.ProgressTimeout)
	if err != nil {
		return false, fmt.Errorf("invalid duration specified for timeout: %w", err)
	}
	currTime := clock.Now()
	failedTime := time.Time{}
	for _, condition := range workspace.Status.Conditions {
		if condition.Type == dw.DevWorkspaceFailedStart {
			failedTime = condition.LastTransitionTime.Time
		}
	}
	if !failedTime.IsZero() && failedTime.Add(timeout).Before(currTime) {
		return true, nil
	}
	return false, nil
}
