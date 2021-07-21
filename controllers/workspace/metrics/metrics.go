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
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	workspaceSourceLabel = "controller.devfile.io/devworkspace-source"
	metricSourceLabel    = "source"
)

var (
	workspaceTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "devworkspace",
			Name:      "started_total",
			Help:      "Number of devworkspace starting events",
		},
		[]string{
			metricSourceLabel,
		},
	)
	workspaceStarts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "devworkspace",
			Name:      "started_success_total",
			Help:      "Number of devworkspaces successfully entering the 'Running' phase",
		},
		[]string{
			metricSourceLabel,
		},
	)
	workspaceFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "devworkspace",
			Name:      "fail_total",
			Help:      "Number of failed DevWorkspaces",
		},
		[]string{
			metricSourceLabel,
		},
	)
	workspaceStartupTimesHist = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "devworkspace",
			Name:      "startup_time",
			Help:      "Total time taken to start a DevWorkspace, in seconds",
			Buckets:   []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160, 170, 180},
		},
		[]string{
			metricSourceLabel,
		},
	)
)

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(workspaceTotal, workspaceStarts, workspaceFailures, workspaceStartupTimesHist)
}
