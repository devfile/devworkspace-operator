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

package v1alpha1

import v1 "k8s.io/api/core/v1"

// Summary of additions that are to be merged into the main workspace deployment
type PodAdditions struct {
	// Annotations to be applied to workspace deployment
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	Annotations map[string]string `json:"annotations,omitempty"`
	// Labels to be applied to workspace deployment
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	Labels map[string]string `json:"labels,omitempty"`
	// Containers to add to workspace deployment
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	Containers []v1.Container `json:"containers,omitempty"`
	// Init containers to add to workspace deployment
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	InitContainers []v1.Container `json:"initContainers,omitempty"`
	// Volumes to add to workspace deployment
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	Volumes []v1.Volume `json:"volumes,omitempty"`
	// VolumeMounts to add to all containers in a workspace deployment
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	VolumeMounts []v1.VolumeMount `json:"volumeMounts,omitempty"`
	// ImagePullSecrets to add to workspace deployment
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	PullSecrets []v1.LocalObjectReference `json:"pullSecrets,omitempty"`
	// Annotations for the workspace service account, it might be used for e.g. OpenShift oauth with SA as auth client
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	ServiceAccountAnnotations map[string]string `json:"serviceAccountAnnotations,omitempty"`
}
