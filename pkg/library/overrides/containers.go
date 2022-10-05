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

package overrides

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/strategicpatch"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

func NeedsContainerOverride(component *dw.Component) bool {
	return component.Container != nil && component.Attributes.Exists(constants.ContainerOverridesAttribute)
}

func ApplyContainerOverrides(component *dw.Component, container *corev1.Container) (*corev1.Container, error) {
	override := &corev1.Container{}
	if err := component.Attributes.GetInto(constants.ContainerOverridesAttribute, override); err != nil {
		return nil, fmt.Errorf("failed to parse %s attribute on component %s: %w", constants.ContainerOverridesAttribute, component.Name, err)
	}
	override = restrictContainerOverride(override)

	overrideBytes, err := json.Marshal(override)
	if err != nil {
		return nil, fmt.Errorf("error applying container overrides: %w", err)
	}

	originalBytes, err := json.Marshal(container)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal container to yaml: %w", err)
	}

	patchedBytes, err := strategicpatch.StrategicMergePatch(originalBytes, overrideBytes, &corev1.Container{})
	if err != nil {
		return nil, fmt.Errorf("failed to apply container overrides: %w", err)
	}

	patched := &corev1.Container{}
	if err := json.Unmarshal(patchedBytes, patched); err != nil {
		return nil, fmt.Errorf("error applying container overrides: %w", err)
	}
	// Applying the patch will overwrite the container's name as corev1.Container.Name
	// does not have the omitempty json tag.
	patched.Name = container.Name
	return patched, nil
}

// restrictContainerOverride unsets fields on a container that should not be
// considered for container overrides. These fields are generally available to
// set as fields on the container component itself.
func restrictContainerOverride(override *corev1.Container) *corev1.Container {
	result := override.DeepCopy()
	result.Name = ""
	result.Image = ""
	result.Command = nil
	result.Args = nil
	result.Ports = nil
	result.VolumeMounts = nil
	result.Env = nil

	return result
}
