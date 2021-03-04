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
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/go-logr/logr"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/workspace/provision"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclock "k8s.io/apimachinery/pkg/util/clock"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type currentStatus struct {
	workspaceConditions
	// Current workspace phase
	phase dw.DevWorkspacePhase
}

func initCurrentStatus() currentStatus {
	return currentStatus{
		workspaceConditions: workspaceConditions{
			conditions: map[dw.DevWorkspaceConditionType]dw.DevWorkspaceCondition{},
		},
		phase: dw.DevWorkspaceStatusStarting,
	}
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
	workspace.Status.Phase = status.phase
	currTransitionTime := metav1.Time{Time: clock.Now()}
	for conditionType, condition := range status.conditions {
		conditionExists := false
		for idx, existingCondition := range workspace.Status.Conditions {
			if existingCondition.Type == conditionType && existingCondition.LastTransitionTime.Before(&currTransitionTime) {
				workspace.Status.Conditions[idx].LastTransitionTime = currTransitionTime
				workspace.Status.Conditions[idx].Status = condition.Status
				workspace.Status.Conditions[idx].Message = condition.Message
				conditionExists = true
				break
			}
		}
		if !conditionExists {
			workspace.Status.Conditions = append(workspace.Status.Conditions, dw.DevWorkspaceCondition{
				Type:               conditionType,
				Message:            condition.Message,
				Status:             condition.Status,
				LastTransitionTime: currTransitionTime,
			})
		}
	}
	for idx, existingCondition := range workspace.Status.Conditions {
		if existingCondition.LastTransitionTime.Before(&currTransitionTime) {
			workspace.Status.Conditions[idx].LastTransitionTime = currTransitionTime
			workspace.Status.Conditions[idx].Status = corev1.ConditionUnknown
			workspace.Status.Conditions[idx].Message = ""
		}
	}
	sort.SliceStable(workspace.Status.Conditions, func(i, j int) bool {
		return strings.Compare(string(workspace.Status.Conditions[i].Type), string(workspace.Status.Conditions[j].Type)) > 0
	})
	infoMessage := getInfoMessage(workspace, status)
	if workspace.Status.Message != infoMessage {
		workspace.Status.Message = infoMessage
	}

	err := r.Status().Update(context.TODO(), workspace)
	if err != nil {
		logger.Info(fmt.Sprintf("Error updating workspace status: %s", err))
		if reconcileError == nil {
			reconcileError = err
		}
	}
	return reconcileResult, reconcileError
}

func syncWorkspaceIdeURL(workspace *dw.DevWorkspace, exposedEndpoints map[string]v1alpha1.ExposedEndpointList, clusterAPI provision.ClusterAPI) (ok bool, err error) {
	ideUrl := getIdeUrl(exposedEndpoints)

	if workspace.Status.IdeUrl == ideUrl {
		return true, nil
	}
	workspace.Status.IdeUrl = ideUrl
	err = clusterAPI.Client.Status().Update(context.TODO(), workspace)
	return false, err
}

func checkServerStatus(workspace *dw.DevWorkspace) (ok bool, err error) {
	ideUrl := workspace.Status.IdeUrl
	if ideUrl == "" {
		// Support DevWorkspaces that do not specify an ideUrl
		return true, nil
	}
	healthz, err := url.Parse(ideUrl)
	if err != nil {
		return false, err
	}
	healthz.Path = healthz.Path + "healthz"

	resp, err := healthHttpClient.Get(healthz.String())
	if err != nil {
		return false, err
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		// Assume endpoint is unimplemented and * is covered with authentication.
		return true, nil
	}
	if resp.StatusCode == 404 {
		// Compatibility: assume endpoint is unimplemented.
		return true, nil
	}
	ok = (resp.StatusCode / 100) == 2
	return ok, nil
}

func getIdeUrl(exposedEndpoints map[string]v1alpha1.ExposedEndpointList) string {
	for _, endpoints := range exposedEndpoints {
		for _, endpoint := range endpoints {
			if endpoint.Attributes.GetString(string(v1alpha1.TypeEndpointAttribute), nil) == "ide" {
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
		if workspace.Status.IdeUrl == "" {
			return "Workspace is running"
		}
		return workspace.Status.IdeUrl
	case dw.DevWorkspaceStatusStopped, dw.DevWorkspaceStatusStopping:
		return string(workspace.Status.Phase)
	}

	latestCondition := status.getFirstFalse()
	if latestCondition != nil {
		return latestCondition.Message
	}

	// No condition is false but workspace is not running; unclear what value should be set.
	return ""
}
