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
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	workspaceSourceLabel     = "controller.devfile.io/devworkspace-source"
	metricSourceLabel        = "source"
	metricsRoutingClassLabel = "routingclass"
	metricsReasonLabel       = "reason"
)

var (
	workspaceTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "devworkspace",
			Name:      "started_total",
			Help:      "Number of DevWorkspace starting events",
		},
		[]string{
			metricSourceLabel,
			metricsRoutingClassLabel,
		},
	)
	workspaceStarts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "devworkspace",
			Name:      "started_success_total",
			Help:      "Number of DevWorkspaces successfully entering the 'Running' phase",
		},
		[]string{
			metricSourceLabel,
			metricsRoutingClassLabel,
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
			metricsReasonLabel,
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
			metricsRoutingClassLabel,
		},
	)
)

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(workspaceTotal, workspaceStarts, workspaceFailures, workspaceStartupTimesHist)
}
