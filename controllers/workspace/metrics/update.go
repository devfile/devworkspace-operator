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

package metrics

import (
	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/devfile/devworkspace-operator/pkg/conditions"
)

// WorkspaceStarted updates metrics for workspaces entering the 'Starting' phase, given a workspace. If an error is
// encountered, the provided logger is used to log the error.
func WorkspaceStarted(wksp *dw.DevWorkspace, log logr.Logger) {
	incrementMetricForWorkspace(workspaceTotal, wksp, log)
}

// WorkspaceRunning updates metrics for workspaces entering the 'Running' phase, given a workspace. If an error is
// encountered, the provided logger is used to log the error. This function assumes the provided workspace has
// fully-synced conditions (i.e. the WorkspaceReady condition is present).
func WorkspaceRunning(wksp *dw.DevWorkspace, log logr.Logger) {
	incrementMetricForWorkspace(workspaceStarts, wksp, log)
	incrementStartTimeBucketForWorkspace(wksp, log)
}

// WorkspaceFailed updates metrics for workspace entering the 'Failed' phase. If an error is encountered, the provided
// logger is used to log the error.
func WorkspaceFailed(wksp *dw.DevWorkspace, log logr.Logger) {
	incrementMetricForWorkspace(workspaceFailures, wksp, log)
}

func incrementMetricForWorkspace(metric *prometheus.CounterVec, wksp *dw.DevWorkspace, log logr.Logger) {
	sourceLabel := wksp.Labels[workspaceSourceLabel]
	ctr, err := metric.GetMetricWith(map[string]string{metricSourceLabel: sourceLabel})
	if err != nil {
		log.Error(err, "Failed to increment metric")
	}
	ctr.Inc()
}

func incrementStartTimeBucketForWorkspace(wksp *dw.DevWorkspace, log logr.Logger) {
	sourceLabel := wksp.Labels[workspaceSourceLabel]
	hist, err := workspaceStartupTimesHist.GetMetricWith(map[string]string{metricSourceLabel: sourceLabel})
	if err != nil {
		log.Error(err, "Failed to update metric")
	}
	readyCondition := conditions.GetConditionByType(wksp.Status.Conditions, dw.DevWorkspaceReady)
	if readyCondition == nil {
		return
	}
	startedCondition := conditions.GetConditionByType(wksp.Status.Conditions, conditions.Started)
	if startedCondition == nil {
		return
	}
	readyTime := readyCondition.LastTransitionTime
	startTime := startedCondition.LastTransitionTime
	startDuration := readyTime.Sub(startTime.Time)
	hist.Observe(startDuration.Seconds())
}
