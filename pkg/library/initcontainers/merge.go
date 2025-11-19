//
// Copyright (c) 2019-2025 Red Hat, Inc.
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

package initcontainers

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// MergeInitContainers performs a strategic merge of init containers.
// Containers with the same name in the patch will be merged into the base,
// and new containers will be appended. The merge uses Kubernetes' strategic
// merge patch semantics with name as the merge key.
func MergeInitContainers(base []corev1.Container, patches []corev1.Container) ([]corev1.Container, error) {
	if len(patches) == 0 {
		return base, nil
	}

	// create PodSpec structure with base init containers
	basePodSpec := corev1.PodSpec{
		InitContainers: base,
	}

	// create PodSpec structure with patch init containers
	patchPodSpec := corev1.PodSpec{
		InitContainers: patches,
	}

	// marshal both structures to JSON
	baseBytes, err := json.Marshal(basePodSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal base init containers: %w", err)
	}
	patchBytes, err := json.Marshal(patchPodSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal patch init containers: %w", err)
	}

	// perform strategic merge patch
	mergedBytes, err := strategicpatch.StrategicMergePatch(
		baseBytes,
		patchBytes,
		&corev1.PodSpec{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to apply strategic merge patch: %w", err)
	}

	// unmarshal the merged result
	var mergedPodSpec corev1.PodSpec
	if err := json.Unmarshal(mergedBytes, &mergedPodSpec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal merged init containers: %w", err)
	}

	/* restore the original containers order */

	// build map for quick lookup
	mergedMap := make(map[string]corev1.Container)
	for _, container := range mergedPodSpec.InitContainers {
		mergedMap[container.Name] = container
	}

	result := make([]corev1.Container, 0, len(mergedPodSpec.InitContainers))
	baseNames := make(map[string]bool)

	// add base containers in order, merged one if patched
	for _, baseContainer := range base {
		baseNames[baseContainer.Name] = true
		if merged, exists := mergedMap[baseContainer.Name]; exists {
			result = append(result, merged)
			delete(mergedMap, baseContainer.Name)
		} else {
			result = append(result, baseContainer)
		}
	}

	// append new containers from patches
	for _, patchContainer := range patches {
		if !baseNames[patchContainer.Name] {
			if merged, exists := mergedMap[patchContainer.Name]; exists {
				result = append(result, merged)
			} else {
				result = append(result, patchContainer)
			}
		}
	}

	return result, nil
}
