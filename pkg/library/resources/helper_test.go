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

package resources

import (
	"testing"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/yaml"
)

func TestParseResourcesFromComponent(t *testing.T) {
	tests := []struct {
		name      string
		component *dw.ContainerComponent
		expected  *corev1.ResourceRequirements
		errRegexp string
	}{
		{
			name:      "Parses all fields in component",
			component: getContainerComponent("1000Mi", "100Mi", "1000m", "100m"),
			expected:  getResourceRequirements("1000Mi", "100Mi", "1000m", "100m"),
		},
		{
			name:      "Returns error when cannot parse memory limit",
			component: getContainerComponent("test", "100Mi", "1000m", "100m"),
			errRegexp: "failed to parse memory limit for container component.*",
		},
		{
			name:      "Returns error when cannot parse memory request",
			component: getContainerComponent("1000Mi", "test", "1000m", "100m"),
			errRegexp: "failed to parse memory request for container component.*",
		},
		{
			name:      "Returns error when cannot parse cpu limit",
			component: getContainerComponent("1000Mi", "100Mi", "test", "100m"),
			errRegexp: "failed to parse CPU limit for container component.*",
		},
		{
			name:      "Returns error when cannot parse cpu request",
			component: getContainerComponent("1000Mi", "100Mi", "1000m", "test"),
			errRegexp: "failed to parse CPU request for container component.*",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			component := &dw.Component{
				Name: "test-component",
			}
			component.Container = tt.component
			actual, err := ParseResourcesFromComponent(component)
			if tt.errRegexp != "" {
				if assert.Error(t, err) {
					assert.Regexp(t, tt.errRegexp, err.Error())
				}
			} else {
				if !assert.NoError(t, err) {
					return
				}
				if !assert.Equal(t, tt.expected, actual) {
					// Print more useful diff since quantity is not a simple struct
					expectedYaml, _ := yaml.Marshal(tt.expected)
					actualYaml, _ := yaml.Marshal(actual)
					t.Logf("\nExpected:\n%s\nActual:\n%s", expectedYaml, actualYaml)
				}
			}
		})
	}
}

func TestAddResourceRequirements(t *testing.T) {
	tests := []struct {
		name      string
		resources *corev1.ResourceRequirements
		toAdd     *corev1.ResourceRequirements
		expected  *corev1.ResourceRequirements
	}{
		{
			name:      "Adds all resources",
			resources: getResourceRequirements("100Mi", "200Mi", "100m", "200m"),
			toAdd:     getResourceRequirements("150Mi", "250Mi", "150m", "250m"),
			expected:  getResourceRequirements("250Mi", "450Mi", "250m", "450m"),
		},
		{
			name:      "Does not add resources if not defined in base",
			resources: getResourceRequirements("", "", "", ""),
			toAdd:     getResourceRequirements("150Mi", "250Mi", "150m", "250m"),
			expected:  getResourceRequirements("", "", "", ""),
		},
		{
			// Not sure if this is ultimately the desired behavior here (maybe depends on usage)
			name:      "Adds resources if zero defined in base",
			resources: getResourceRequirements("0", "0", "0", "0"),
			toAdd:     getResourceRequirements("150Mi", "250Mi", "150m", "250m"),
			expected:  getResourceRequirements("150Mi", "250Mi", "150m", "250m"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := AddResourceRequirements(tt.resources, tt.toAdd)
			// Need to compare marshalled representations as quantities hold internal state (string representation
			// is cached for quicker retrieval)
			expectedYaml, _ := yaml.Marshal(tt.expected)
			expectedStr := string(expectedYaml)
			actualYaml, _ := yaml.Marshal(actual)
			actualStr := string(actualYaml)
			assert.Equal(t, expectedStr, actualStr, "\nExpected:\n%s\nActual:\n%s", expectedYaml, actualYaml)
		})
	}
}

func TestApplyResourceRequirementsToComponent(t *testing.T) {
	tests := []struct {
		name              string
		container         *dw.ContainerComponent
		resources         *corev1.ResourceRequirements
		expectedContainer *dw.ContainerComponent
	}{
		{
			name:              "Overwrites all resources",
			container:         getContainerComponent("2000Mi", "200Mi", "200m", "2000m"),
			resources:         getResourceRequirements("100Mi", "100Mi", "100m", "100m"),
			expectedContainer: getContainerComponent("100Mi", "100Mi", "100m", "100m"),
		},
		{
			name:              "Does not apply zero values in defaults",
			container:         getContainerComponent("2000Mi", "200Mi", "200m", "2000m"),
			resources:         getResourceRequirements("0", "0", "0", "0"),
			expectedContainer: getContainerComponent("2000Mi", "200Mi", "200m", "2000m"),
		},
		{
			name:              "Handles unset fields in defaults",
			container:         getContainerComponent("2000Mi", "200Mi", "200m", "2000m"),
			resources:         getResourceRequirements("", "", "", ""),
			expectedContainer: getContainerComponent("2000Mi", "200Mi", "200m", "2000m"),
		},
		{
			name:              "Handles nil requests and limits in defaults",
			container:         getContainerComponent("2000Mi", "200Mi", "200m", "2000m"),
			resources:         &corev1.ResourceRequirements{},
			expectedContainer: getContainerComponent("2000Mi", "200Mi", "200m", "2000m"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ApplyResourceRequirementsToComponent(tt.container, tt.resources)
			assert.Equal(t, tt.expectedContainer, tt.container)
		})
	}
}

func TestFilterResources(t *testing.T) {
	tests := []struct {
		name      string
		resources *corev1.ResourceRequirements
		expected  *corev1.ResourceRequirements
	}{
		{
			name:      "Does not modify non-zero resources",
			resources: getResourceRequirements("1000Mi", "100Mi", "1000m", "100m"),
			expected:  getResourceRequirements("1000Mi", "100Mi", "1000m", "100m"),
		},
		{
			name:      "Removes zero memory limit",
			resources: getResourceRequirements("0", "100Mi", "1000m", "100m"),
			expected:  getResourceRequirements("", "100Mi", "1000m", "100m"),
		},
		{
			name:      "Removes zero memory request",
			resources: getResourceRequirements("1000Mi", "0", "1000m", "100m"),
			expected:  getResourceRequirements("1000Mi", "", "1000m", "100m"),
		},
		{
			name:      "Removes zero cpu limit",
			resources: getResourceRequirements("1000Mi", "100Mi", "0", "100m"),
			expected:  getResourceRequirements("1000Mi", "100Mi", "", "100m"),
		},
		{
			name:      "Removes zero cpu request",
			resources: getResourceRequirements("1000Mi", "100Mi", "1000m", "0"),
			expected:  getResourceRequirements("1000Mi", "100Mi", "1000m", ""),
		},
		{
			name:      "Removes requests entirely when all zero",
			resources: getResourceRequirements("1000Mi", "0", "1000m", "0"),
			expected: &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("1000Mi"),
					corev1.ResourceCPU:    resource.MustParse("1000m"),
				},
			},
		},
		{
			name:      "Removes limits entirely when all zero",
			resources: getResourceRequirements("0", "100Mi", "0", "100m"),
			expected: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("100Mi"),
					corev1.ResourceCPU:    resource.MustParse("100m"),
				},
			},
		},
		{
			name:      "Returns empty resources when all fields are zero",
			resources: getResourceRequirements("0", "0", "0", "0"),
			expected:  &corev1.ResourceRequirements{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := FilterResources(tt.resources)
			if !assert.Equal(t, tt.expected, actual) {
				// Print more useful diff since quantity is not a simple struct
				expectedYaml, _ := yaml.Marshal(tt.expected)
				actualYaml, _ := yaml.Marshal(actual)
				t.Logf("\nExpected:\n%s\nActual:\n%s", expectedYaml, actualYaml)
			}
		})
	}

}

func TestApplyDefaults(t *testing.T) {
	tests := []struct {
		name     string
		base     *corev1.ResourceRequirements
		defaults *corev1.ResourceRequirements
		expected *corev1.ResourceRequirements
	}{
		{
			name:     "Applies all defaults to empty resources",
			base:     &corev1.ResourceRequirements{},
			defaults: getResourceRequirements("2000Mi", "200Mi", "2000m", "200m"),
			expected: getResourceRequirements("2000Mi", "200Mi", "2000m", "200m"),
		},
		{
			name:     "Does not overwrite nonempty fields",
			base:     getResourceRequirements("1000Mi", "100Mi", "1000m", "100m"),
			defaults: getResourceRequirements("2000Mi", "200Mi", "2000m", "200m"),
			expected: getResourceRequirements("1000Mi", "100Mi", "1000m", "100m"),
		},
		{
			name:     "Does not apply empty default fields",
			base:     getResourceRequirements("1000Mi", "", "1000m", "100m"),
			defaults: getResourceRequirements("2000Mi", "", "2000m", ""),
			expected: getResourceRequirements("1000Mi", "", "1000m", "100m"),
		},
		{
			name:     "Handles nil defaults",
			base:     getResourceRequirements("1000Mi", "100Mi", "1000m", "100m"),
			defaults: nil,
			expected: getResourceRequirements("1000Mi", "100Mi", "1000m", "100m"),
		},
		{
			name:     "Ignores '0' fields in defaults",
			base:     getResourceRequirements("", "", "", ""),
			defaults: getResourceRequirements("0", "0", "0", "0"),
			expected: getResourceRequirements("", "", "", ""),
		},
		{
			name:     "Doesn't set invalid default memory and cpu limit (uses request instead of default)",
			base:     getResourceRequirements("", "1000Mi", "", "1000m"),
			defaults: getResourceRequirements("500Mi", "", "500m", ""),
			expected: getResourceRequirements("1000Mi", "1000Mi", "1000m", "1000m"),
		},
		{
			name:     "Doesn't set invalid default memory and cpu request (uses limit instead of default)",
			base:     getResourceRequirements("100Mi", "", "100m", ""),
			defaults: getResourceRequirements("", "500Mi", "", "500m"),
			expected: getResourceRequirements("100Mi", "100Mi", "100m", "100m"),
		},
		{
			name:     "Does not modify invalid resources",
			base:     getResourceRequirements("100Mi", "1000Mi", "100m", "1000m"),
			defaults: getResourceRequirements("1000Mi", "1000Mi", "1000m", "1000m"),
			expected: getResourceRequirements("100Mi", "1000Mi", "100m", "1000m"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := ApplyDefaults(tt.base, tt.defaults)
			if !assert.Equal(t, tt.expected, actual) {
				// Print more useful diff since quantity is not a simple struct
				expectedYaml, _ := yaml.Marshal(tt.expected)
				actualYaml, _ := yaml.Marshal(actual)
				t.Logf("\nExpected:\n%s\nActual:\n%s", expectedYaml, actualYaml)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		resources *corev1.ResourceRequirements
		errRegexp string
	}{
		{
			name:      "Valid resources",
			resources: getResourceRequirements("1000Mi", "100Mi", "1000m", "100m"),
			errRegexp: "",
		},
		{
			name:      "Invalid memory request",
			resources: getResourceRequirements("100Mi", "200Mi", "1000m", "100m"),
			errRegexp: "memory request \\(200Mi\\) must be less than or equal to limit \\(100Mi\\)",
		},
		{
			name:      "Invalid cpu request",
			resources: getResourceRequirements("1000Mi", "100Mi", "100m", "200m"),
			errRegexp: "CPU request \\(200m\\) must be less than or equal to limit \\(100m\\)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateResources(tt.resources)
			if tt.errRegexp == "" {
				assert.NoError(t, err)
			} else {
				if assert.Error(t, err) {
					assert.Regexp(t, tt.errRegexp, err.Error())
				}
			}
		})
	}
}

func getResourceRequirements(memLimit, memRequest, cpuLimit, cpuRequest string) *corev1.ResourceRequirements {
	reqs := &corev1.ResourceRequirements{
		Limits:   corev1.ResourceList{},
		Requests: corev1.ResourceList{},
	}
	if memLimit != "" {
		reqs.Limits[corev1.ResourceMemory] = resource.MustParse(memLimit)
	}
	if memRequest != "" {
		reqs.Requests[corev1.ResourceMemory] = resource.MustParse(memRequest)
	}
	if cpuLimit != "" {
		reqs.Limits[corev1.ResourceCPU] = resource.MustParse(cpuLimit)
	}
	if cpuRequest != "" {
		reqs.Requests[corev1.ResourceCPU] = resource.MustParse(cpuRequest)
	}

	return reqs
}

func getContainerComponent(memLimit, memRequest, cpuLimit, cpuRequest string) *dw.ContainerComponent {
	container := &dw.ContainerComponent{}
	container.MemoryLimit = memLimit
	container.MemoryRequest = memRequest
	container.CpuLimit = cpuLimit
	container.CpuRequest = cpuRequest
	return container
}
