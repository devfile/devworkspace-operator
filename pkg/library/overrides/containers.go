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

package overrides

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/strategicpatch"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

// Constants representing the default values used by Kubernetes for container fields.
// If a pod is created that does not set these fields in a Probe (but does create a probe)
// these are the values that will be automatically applied to the pod spec.
const (
	defaultProbeSuccessThreshold = 1
	defaultProbeFailureThreshold = 3
	defaultProbeTimeoutSeconds   = 1
	defaultProbePeriodSeconds    = 10
)

func NeedsContainerOverride(component *dw.Component) bool {
	return component.Container != nil && component.Attributes.Exists(constants.ContainerOverridesAttribute)
}

func ApplyContainerOverrides(component *dw.Component, container *corev1.Container) (*corev1.Container, error) {
	override := &corev1.Container{}
	if err := component.Attributes.GetInto(constants.ContainerOverridesAttribute, override); err != nil {
		return nil, fmt.Errorf("failed to parse %s attribute on component %s: %w", constants.ContainerOverridesAttribute, component.Name, err)
	}
	if err := restrictContainerOverride(override); err != nil {
		return nil, fmt.Errorf("failed to parse %s attribute on component %s: %w", constants.ContainerOverridesAttribute, component.Name, err)
	}

	overrideJSON := component.Attributes[constants.ContainerOverridesAttribute]

	originalBytes, err := json.Marshal(container)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal container to yaml: %w", err)
	}

	patchedBytes, err := strategicpatch.StrategicMergePatch(originalBytes, overrideJSON.Raw, &corev1.Container{})
	if err != nil {
		return nil, fmt.Errorf("failed to apply container overrides: %w", err)
	}

	patched := &corev1.Container{}
	if err := json.Unmarshal(patchedBytes, patched); err != nil {
		return nil, fmt.Errorf("error applying container overrides: %w", err)
	}
	// Applying the patch will overwrite the container's name and image as corev1.Container.Name
	// does not have the omitempty json tag.
	patched.Name = container.Name
	patched.Image = container.Image

	// Make sure any fields that will be defaulted by the cluster are set (e.g. probes)
	handleDefaultedContainerFields(patched)

	return patched, nil
}

// restrictContainerOverride unsets fields on a container that should not be
// considered for container overrides. These fields are generally available to
// set as fields on the container component itself.
func restrictContainerOverride(override *corev1.Container) error {
	invalidField := ""
	if override.Name != "" {
		invalidField = "name"
	}
	if override.Image != "" {
		invalidField = "image"
	}
	if override.Command != nil {
		invalidField = "command"
	}
	if override.Args != nil {
		invalidField = "args"
	}
	if override.Ports != nil {
		invalidField = "ports"
	}
	if override.VolumeMounts != nil {
		invalidField = "volumeMounts"
	}
	if override.Env != nil {
		invalidField = "env"
	}
	if invalidField != "" {
		return fmt.Errorf("cannot use container-overrides to override container %s", invalidField)
	}
	return nil
}

// handleDefaultedContainerFields fills partially-filled structs with defaulted fields
// in a container. This is required to avoid repeatedly reconciling a container where e.g.
//
// 1. We create a readinessProbe with no successThreshold since it's not defined in the override
// 2. Kubernetes sets the defaulted field successThreshold: 1
// 3. We detect that the spec is different from the cluster and unset successThreshold (go to 1.)
func handleDefaultedContainerFields(patched *corev1.Container) {
	setProbeFields := func(probe *corev1.Probe) {
		if probe.SuccessThreshold == 0 {
			probe.SuccessThreshold = defaultProbeSuccessThreshold
		}
		if probe.FailureThreshold == 0 {
			probe.FailureThreshold = defaultProbeFailureThreshold
		}
		if probe.PeriodSeconds == 0 {
			probe.PeriodSeconds = defaultProbePeriodSeconds
		}
		if probe.TimeoutSeconds == 0 {
			probe.TimeoutSeconds = defaultProbeTimeoutSeconds
		}
	}
	if patched.ReadinessProbe != nil {
		setProbeFields(patched.ReadinessProbe)
	}
	if patched.LivenessProbe != nil {
		setProbeFields(patched.LivenessProbe)
	}
	if patched.StartupProbe != nil {
		setProbeFields(patched.StartupProbe)
	}
}
