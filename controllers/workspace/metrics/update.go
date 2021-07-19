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
)

func WorkspaceStarted(wksp *dw.DevWorkspace, log logr.Logger) {
	incrementMetricForWorkspace(workspaceTotal, wksp, log)
}

func WorkspaceRunning(wksp *dw.DevWorkspace, log logr.Logger) {
	incrementMetricForWorkspace(workspaceStarts, wksp, log)
}

func WorkspaceFailed(wksp *dw.DevWorkspace, log logr.Logger) {
	incrementMetricForWorkspace(workspaceFailures, wksp, log)
}

func incrementMetricForWorkspace(metric *prometheus.CounterVec, wksp *dw.DevWorkspace, log logr.Logger) {
	sourceLabel := wksp.Labels[workspaceSourceLabel]
	ctr, err := metric.GetMetricWith(map[string]string{metricSourceLabel: sourceLabel})
	if err != nil {
		log.Info("Failed to increment metric: %s", err)
	}
	ctr.Inc()
}
