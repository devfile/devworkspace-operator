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

package metrics

import (
	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/conditions"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

// WorkspaceStarted updates metrics for workspaces entering the 'Starting' phase, given a workspace. If an error is
// encountered, the provided logger is used to log the error.
func WorkspaceStarted(wksp *common.DevWorkspaceWithConfig, log logr.Logger) {
	_, ok := wksp.GetAnnotations()[constants.DevWorkspaceStartedAtAnnotation]
	if !ok {
		incrementMetricForWorkspace(workspaceTotal, wksp, log)
	}
}

// WorkspaceRunning updates metrics for workspaces entering the 'Running' phase, given a workspace. If an error is
// encountered, the provided logger is used to log the error. This function assumes the provided workspace has
// fully-synced conditions (i.e. the WorkspaceReady condition is present).
func WorkspaceRunning(wksp *common.DevWorkspaceWithConfig, log logr.Logger) {
	_, ok := wksp.GetAnnotations()[constants.DevWorkspaceStartedAtAnnotation]
	if !ok {
		incrementMetricForWorkspace(workspaceStarts, wksp, log)
		incrementStartTimeBucketForWorkspace(wksp, log)
	}
}

// WorkspaceFailed updates metrics for workspace entering the 'Failed' phase. If an error is encountered, the provided
// logger is used to log the error.
func WorkspaceFailed(wksp *common.DevWorkspaceWithConfig, log logr.Logger) {
	incrementMetricForWorkspaceFailure(workspaceFailures, wksp, log)
}

func incrementMetricForWorkspace(metric *prometheus.CounterVec, workspace *common.DevWorkspaceWithConfig, log logr.Logger) {
	sourceLabel := workspace.Labels[workspaceSourceLabel]
	if sourceLabel == "" {
		sourceLabel = "unknown"
	}
	routingClass := workspace.Spec.RoutingClass
	if routingClass == "" {
		routingClass = workspace.Config.Routing.DefaultRoutingClass
	}
	ctr, err := metric.GetMetricWith(map[string]string{metricSourceLabel: sourceLabel, metricsRoutingClassLabel: routingClass})
	if err != nil {
		log.Error(err, "Failed to increment metric")
	}
	ctr.Inc()
}

func incrementMetricForWorkspaceFailure(metric *prometheus.CounterVec, workspace *common.DevWorkspaceWithConfig, log logr.Logger) {
	sourceLabel := workspace.Labels[workspaceSourceLabel]
	if sourceLabel == "" {
		sourceLabel = "unknown"
	}
	reason := GetFailureReason(workspace)
	ctr, err := metric.GetMetricWith(map[string]string{metricSourceLabel: sourceLabel, metricsReasonLabel: string(reason)})
	if err != nil {
		log.Error(err, "Failed to increment metric")
	}
	ctr.Inc()
}

func incrementStartTimeBucketForWorkspace(workspace *common.DevWorkspaceWithConfig, log logr.Logger) {
	sourceLabel := workspace.Labels[workspaceSourceLabel]
	if sourceLabel == "" {
		sourceLabel = "unknown"
	}
	routingClass := workspace.Spec.RoutingClass
	if routingClass == "" {
		routingClass = workspace.Config.Routing.DefaultRoutingClass
	}
	hist, err := workspaceStartupTimesHist.GetMetricWith(map[string]string{metricSourceLabel: sourceLabel, metricsRoutingClassLabel: routingClass})
	if err != nil {
		log.Error(err, "Failed to update metric")
	}
	readyCondition := conditions.GetConditionByType(workspace.Status.Conditions, dw.DevWorkspaceReady)
	if readyCondition == nil {
		return
	}
	startedCondition := conditions.GetConditionByType(workspace.Status.Conditions, conditions.Started)
	if startedCondition == nil {
		return
	}
	readyTime := readyCondition.LastTransitionTime
	startTime := startedCondition.LastTransitionTime
	startDuration := readyTime.Sub(startTime.Time)
	hist.Observe(startDuration.Seconds())
}
