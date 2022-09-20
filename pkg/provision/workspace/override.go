// Copyright (c) 2019-2022 Red Hat, Inc.
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

package workspace

import (
	"github.com/devfile/devworkspace-operator/pkg/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

func needsPodSpecOverrides(workspace *common.DevWorkspaceWithConfig) bool {
	return workspace.Spec.PodSpecOverrides != nil
}

func applyPodSpecOverrides(deployment *appsv1.Deployment, workspace *common.DevWorkspaceWithConfig) (*appsv1.Deployment, error) {
	patched := deployment.DeepCopy()

	originalBytes, err := json.Marshal(patched.Spec.Template)
	if err != nil {
		return nil, err
	}

	podSpecOverride := workspace.Spec.PodSpecOverrides
	patchBytes, err := json.Marshal(podSpecOverride)
	if err != nil {
		return nil, err
	}

	patchedJSON, err := strategicpatch.StrategicMergePatch(originalBytes, patchBytes, &corev1.PodTemplateSpec{})
	if err != nil {
		return nil, err
	}

	patchedPodSpecTemplate := corev1.PodTemplateSpec{}
	if err := json.Unmarshal(patchedJSON, &patchedPodSpecTemplate); err != nil {
		return nil, err
	}
	patched.Spec.Template = patchedPodSpecTemplate
	return patched, nil
}
