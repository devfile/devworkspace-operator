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

package handler

import (
	"fmt"
	"sort"
	"strings"

	dwv2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
)

type unsupportedWarnings struct {
	serviceAnnotations  map[string]bool
	endpointAnnotations map[string]bool
	dedicatedPod        map[string]bool
	imageComponent      map[string]bool
	customComponent     map[string]bool
	eventPostStop       map[string]bool
}

// Returns an initialized unsupportedWarnings struct
func newUnsupportedWarnings() *unsupportedWarnings {
	return &unsupportedWarnings{
		serviceAnnotations:  make(map[string]bool),
		endpointAnnotations: make(map[string]bool),
		dedicatedPod:        make(map[string]bool),
		imageComponent:      make(map[string]bool),
		customComponent:     make(map[string]bool),
		eventPostStop:       make(map[string]bool),
	}
}

func checkUnsupportedFeatures(devWorkspaceSpec dwv2.DevWorkspaceTemplateSpec) (warnings *unsupportedWarnings) {
	warnings = newUnsupportedWarnings()
	for _, component := range devWorkspaceSpec.Components {
		if component.Container != nil {
			if component.Container.Annotation != nil && component.Container.Annotation.Service != nil {
				warnings.serviceAnnotations[component.Name] = true
			}
			for _, endpoint := range component.Container.Endpoints {
				if endpoint.Annotations != nil {
					warnings.endpointAnnotations[component.Name] = true
				}
			}
			if component.Container.DedicatedPod != nil && *component.Container.DedicatedPod {
				warnings.dedicatedPod[component.Name] = true
			}
		}

		if component.Image != nil {
			warnings.imageComponent[component.Name] = true
		}
		if component.Custom != nil {
			warnings.customComponent[component.Name] = true
		}
	}
	if devWorkspaceSpec.Events != nil {
		if len(devWorkspaceSpec.Events.PostStop) > 0 {
			for _, event := range devWorkspaceSpec.Events.PostStop {
				warnings.eventPostStop[event] = true
			}
		}
	}
	return warnings
}

func unsupportedWarningsPresent(warnings *unsupportedWarnings) bool {
	return len(warnings.serviceAnnotations) > 0 ||
		len(warnings.endpointAnnotations) > 0 ||
		len(warnings.dedicatedPod) > 0 ||
		len(warnings.imageComponent) > 0 ||
		len(warnings.customComponent) > 0 ||
		len(warnings.eventPostStop) > 0
}

func formatUnsupportedFeaturesWarning(warnings *unsupportedWarnings) string {
	var msg []string

	// Returns warning names in sorted order
	getWarningNames := func(warningsMap map[string]bool) []string {
		var warningNames []string
		for name := range warningsMap {
			warningNames = append(warningNames, name)
		}
		sort.Strings(warningNames)
		return warningNames
	}

	if len(warnings.serviceAnnotations) > 0 {
		serviceAnnotationsMsg := "components[].container.annotation.service, used by components: " + strings.Join(getWarningNames(warnings.serviceAnnotations), ", ")
		msg = append(msg, serviceAnnotationsMsg)
	}
	if len(warnings.endpointAnnotations) > 0 {
		endpointAnnotationsMsg := "components[].container.endpoints[].annotations, used by components: " + strings.Join(getWarningNames(warnings.endpointAnnotations), ", ")
		msg = append(msg, endpointAnnotationsMsg)
	}
	if len(warnings.dedicatedPod) > 0 {
		dedicatedPodMsg := "components[].container.dedicatedPod, used by components: " + strings.Join(getWarningNames(warnings.dedicatedPod), ", ")
		msg = append(msg, dedicatedPodMsg)
	}
	if len(warnings.imageComponent) > 0 {
		imageComponentMsg := "components[].image, used by components: " + strings.Join(getWarningNames(warnings.imageComponent), ", ")
		msg = append(msg, imageComponentMsg)
	}
	if len(warnings.customComponent) > 0 {
		customComponentMsg := "components[].custom, used by components: " + strings.Join(getWarningNames(warnings.customComponent), ", ")
		msg = append(msg, customComponentMsg)
	}
	if len(warnings.eventPostStop) > 0 {
		eventPostStopMsg := "events.postStop: " + strings.Join(getWarningNames(warnings.eventPostStop), ", ")
		msg = append(msg, eventPostStopMsg)
	}
	return fmt.Sprintf("Unsupported Devfile features are present in this workspace. The following features will have no effect: %s", strings.Join(msg, "; "))
}

// Returns unsupported feature warnings that are present in the new workspace
// but not present in the old workspace
func checkForAddedUnsupportedFeatures(oldWksp, newWksp *dwv2.DevWorkspace) *unsupportedWarnings {
	oldWarnings := checkUnsupportedFeatures(oldWksp.Spec.Template)
	newWarnings := checkUnsupportedFeatures(newWksp.Spec.Template)
	addedWarnings := newUnsupportedWarnings()

	getAddedWarnings := func(old, new map[string]bool) map[string]bool {
		newWarningNames := make(map[string]bool)
		for name := range new {
			if !old[name] {
				newWarningNames[name] = true
			}
		}
		return newWarningNames
	}

	addedWarnings.serviceAnnotations = getAddedWarnings(oldWarnings.serviceAnnotations, newWarnings.serviceAnnotations)
	addedWarnings.endpointAnnotations = getAddedWarnings(oldWarnings.endpointAnnotations, newWarnings.endpointAnnotations)
	addedWarnings.dedicatedPod = getAddedWarnings(oldWarnings.dedicatedPod, newWarnings.dedicatedPod)
	addedWarnings.imageComponent = getAddedWarnings(oldWarnings.imageComponent, newWarnings.imageComponent)
	addedWarnings.customComponent = getAddedWarnings(oldWarnings.customComponent, newWarnings.customComponent)
	addedWarnings.eventPostStop = getAddedWarnings(oldWarnings.eventPostStop, newWarnings.eventPostStop)
	return addedWarnings
}
