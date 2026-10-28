// Copyright (c) 2019-2026 Red Hat, Inc.
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

package common

import (
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// GetWorkspacePodAffinity returns the pod affinity to apply to pods created for a DevWorkspace,
// combining the podAffinity and podAntiAffinity rules configured on the workspace configuration.
// Returns nil if neither is configured, so that no affinity is set on the pod.
// The returned affinity never sets nodeAffinity: node affinity is computed by the operator
// (e.g. to target the node a PVC is attached to) and must not be overridden by configuration.
func GetWorkspacePodAffinity(config *controllerv1alpha1.WorkspaceConfig) *corev1.Affinity {
	if config == nil || (config.PodAffinity == nil && config.PodAntiAffinity == nil) {
		return nil
	}
	return &corev1.Affinity{
		PodAffinity:     config.PodAffinity,
		PodAntiAffinity: config.PodAntiAffinity,
	}
}

// GetWorkspacePodAffinityWithNodeAffinity returns the pod affinity to apply to pods created for
// a DevWorkspace, combining the podAffinity and podAntiAffinity rules configured on the workspace
// configuration with the operator-computed nodeAffinity (e.g. to target the node a PVC is attached
// to)
func GetWorkspacePodAffinityWithNodeAffinity(config *controllerv1alpha1.WorkspaceConfig, nodeAffinity *corev1.NodeAffinity) *corev1.Affinity {
	if config == nil || (config.PodAffinity == nil && config.PodAntiAffinity == nil) {
		if nodeAffinity == nil {
			return nil
		}
		return &corev1.Affinity{
			NodeAffinity: nodeAffinity,
		}
	}

	return &corev1.Affinity{
		PodAffinity:     config.PodAffinity,
		PodAntiAffinity: config.PodAntiAffinity,
		NodeAffinity:    nodeAffinity,
	}
}
