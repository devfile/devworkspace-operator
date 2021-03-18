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

	devworkspace "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/workspace/provision"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclock "k8s.io/apimachinery/pkg/util/clock"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const PullSecretsReadyCondition devworkspace.WorkspaceConditionType = "PullSecretsReady"

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
func (r *DevWorkspaceReconciler) updateWorkspaceStatus(workspace *devworkspace.DevWorkspace, logger logr.Logger, status *currentStatus, reconcileResult reconcile.Result, reconcileError error) (reconcile.Result, error) {
	workspace.Status.Phase = status.Phase
	currTransitionTime := metav1.Time{Time: clock.Now()}
	for conditionType, conditionMsg := range status.Conditions {
		conditionExists := false
		for idx, condition := range workspace.Status.Conditions {
			if condition.Type == conditionType && condition.LastTransitionTime.Before(&currTransitionTime) {
				workspace.Status.Conditions[idx].LastTransitionTime = currTransitionTime
				workspace.Status.Conditions[idx].Status = corev1.ConditionTrue
				workspace.Status.Conditions[idx].Message = conditionMsg
				conditionExists = true
				break
			}
		}
		if !conditionExists {
			workspace.Status.Conditions = append(workspace.Status.Conditions, devworkspace.WorkspaceCondition{
				Type:               conditionType,
				Message:            conditionMsg,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: currTransitionTime,
			})
		}
	}
	for idx, condition := range workspace.Status.Conditions {
		if condition.LastTransitionTime.Before(&currTransitionTime) {
			workspace.Status.Conditions[idx].LastTransitionTime = currTransitionTime
			workspace.Status.Conditions[idx].Status = corev1.ConditionUnknown
			workspace.Status.Conditions[idx].Message = ""
		}
	}
	sort.SliceStable(workspace.Status.Conditions, func(i, j int) bool {
		return strings.Compare(string(workspace.Status.Conditions[i].Type), string(workspace.Status.Conditions[j].Type)) > 0
	})
	infoMessage := getInfoMessage(workspace, status.Conditions)
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

func syncWorkspaceIdeURL(workspace *devworkspace.DevWorkspace, exposedEndpoints map[string]v1alpha1.ExposedEndpointList, clusterAPI provision.ClusterAPI) (ok bool, err error) {
	ideUrl := getIdeUrl(exposedEndpoints)

	if workspace.Status.IdeUrl == ideUrl {
		return true, nil
	}
	workspace.Status.IdeUrl = ideUrl
	err = clusterAPI.Client.Status().Update(context.TODO(), workspace)
	return false, err
}

func checkServerStatus(workspace *devworkspace.DevWorkspace) (ok bool, err error) {
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

type PhaseStatus struct {
	Condition devworkspace.WorkspaceConditionType
	Message   string
}

// Defines the order in which DevWorkspace Controller processes phases on reconcile
// The last unready condition should be shown
var PhaseStatusesOrder = []PhaseStatus{
	{
		devworkspace.WorkspaceComponentsReady,
		"Processing DevWorkspace components",
	},
	{
		devworkspace.WorkspaceRoutingReady,
		"Waiting for workspace routing objects",
	},
	{
		devworkspace.WorkspaceServiceAccountReady,
		"Waiting for workspace serviceaccount",
	},
	{
		PullSecretsReadyCondition,
		"Waiting for workspace pull secrets",
	},
	{
		devworkspace.WorkspaceReady,
		"Waiting for deployment to be ready",
	},
}

func getInfoMessage(workspace *devworkspace.DevWorkspace, conditions map[devworkspace.WorkspaceConditionType]string) string {
	// Check for errors and failure
	if msg, ok := conditions[devworkspace.WorkspaceError]; ok {
		return msg
	}
	if msg, ok := conditions[devworkspace.WorkspaceFailedStart]; ok {
		return msg
	}
	switch workspace.Status.Phase {
	case devworkspace.WorkspaceStatusRunning:
		if workspace.Status.IdeUrl == "" {
			return "Workspace is running"
		}
		return workspace.Status.IdeUrl
	case devworkspace.WorkspaceStatusStopped, devworkspace.WorkspaceStatusStopping:
		return string(workspace.Status.Phase)
	}

	for _, phaseStatus := range PhaseStatusesOrder {
		if _, present := conditions[phaseStatus.Condition]; !present {
			return phaseStatus.Message
		}
	}

	//All listed conditions are true, if workspace is not started yet, it's due editor server health check
	return "Waiting on editor to start"
}
