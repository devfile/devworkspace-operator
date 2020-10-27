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

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha1"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/workspace/provision"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclock "k8s.io/apimachinery/pkg/util/clock"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

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
		return false, nil
	}
	healthz, err := url.Parse(ideUrl)
	if err != nil {
		return false, err
	}
	healthz.Path = "healthz"

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
			if endpoint.Attributes[string(v1alpha1.TYPE_ENDPOINT_ATTRIBUTE)] == "ide" {
				return endpoint.Url
			}
		}
	}
	return ""
}
