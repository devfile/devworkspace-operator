//
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
//
package v1alpha1

import v1 "k8s.io/api/core/v1"

// Summary of additions that are to be merged into the main devworkspace deployment
type PodAdditions struct {
	// Annotations to be applied to devworkspace deployment
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	Annotations map[string]string `json:"annotations,omitempty"`
	// Labels to be applied to devworkspace deployment
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	Labels map[string]string `json:"labels,omitempty"`
	// Containers to add to devworkspace deployment
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	Containers []v1.Container `json:"containers,omitempty"`
	// Init containers to add to devworkspace deployment
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	InitContainers []v1.Container `json:"initContainers,omitempty"`
	// Volumes to add to devworkspace deployment
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	Volumes []v1.Volume `json:"volumes,omitempty"`
	// VolumeMounts to add to all containers in a devworkspace deployment
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	VolumeMounts []v1.VolumeMount `json:"volumeMounts,omitempty"`
	// ImagePullSecrets to add to devworkspace deployment
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	PullSecrets []v1.LocalObjectReference `json:"pullSecrets,omitempty"`
	// Annotations for the devworkspace service account, it might be used for e.g. OpenShift oauth with SA as auth client
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	ServiceAccountAnnotations map[string]string `json:"serviceAccountAnnotations,omitempty"`
}
