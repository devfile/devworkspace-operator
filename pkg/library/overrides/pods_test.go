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
	"os"
	"path/filepath"
	"testing"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

func TestApplyPodOverrides(t *testing.T) {
	tests := loadAllPodTestCasesOrPanic(t, "testdata/pod-overrides")
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s (%s)", tt.Name, tt.originalFilename), func(t *testing.T) {
			workspace := &common.DevWorkspaceWithConfig{}
			workspace.DevWorkspace = &dw.DevWorkspace{}
			workspace.Spec.Template = *tt.Input.Workspace
			deploy := &appsv1.Deployment{}
			deploy.Spec.Template = *tt.Input.PodTemplateSpec
			actualDeploy, err := ApplyPodOverrides(workspace, deploy)
			if tt.Output.ErrRegexp != nil && assert.Error(t, err) {
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error(), "Error message should match")
			} else {
				if !assert.NoError(t, err, "Should not return error") {
					return
				}
				assert.Truef(t, cmp.Equal(tt.Output.PodTemplateSpec, &actualDeploy.Spec.Template),
					"Deployment should match expected output:\n%s",
					cmp.Diff(tt.Output.PodTemplateSpec, &actualDeploy.Spec.Template))
			}
		})
	}
}

func TestNeedsPodOverride(t *testing.T) {
	jsonPodOverrides := apiext.JSON{
		Raw: []byte(`{"spec":{"runtimeClassName":"kata"}}`),
	}
	tests := []struct {
		Name     string
		Input    dw.DevWorkspaceTemplateSpec
		Expected bool
	}{
		{
			Name:     "Empty workspace does not need override",
			Input:    dw.DevWorkspaceTemplateSpec{},
			Expected: false,
		},
		{
			Name: "Workspace with no overrides",
			Input: dw.DevWorkspaceTemplateSpec{
				DevWorkspaceTemplateSpecContent: dw.DevWorkspaceTemplateSpecContent{
					Components: []dw.Component{
						{
							Name: "test-component",
							ComponentUnion: dw.ComponentUnion{
								Container: &dw.ContainerComponent{
									Container: dw.Container{
										Image: "test-image",
									},
								},
							},
						},
					},
				},
			},
			Expected: false,
		},
		{
			Name: "Workspace with overrides in container",
			Input: dw.DevWorkspaceTemplateSpec{
				DevWorkspaceTemplateSpecContent: dw.DevWorkspaceTemplateSpecContent{
					Components: []dw.Component{
						{
							Name: "test-component",
							Attributes: attributes.Attributes{
								constants.PodOverridesAttribute: jsonPodOverrides,
							},
							ComponentUnion: dw.ComponentUnion{
								Container: &dw.ContainerComponent{
									Container: dw.Container{
										Image: "test-image",
									},
								},
							},
						},
					},
				},
			},
			Expected: true,
		},
		{
			Name: "Workspace with overrides in template",
			Input: dw.DevWorkspaceTemplateSpec{
				DevWorkspaceTemplateSpecContent: dw.DevWorkspaceTemplateSpecContent{
					Attributes: attributes.Attributes{
						constants.PodOverridesAttribute: jsonPodOverrides,
					},
					Components: []dw.Component{
						{
							Name: "test-component",
							ComponentUnion: dw.ComponentUnion{
								Container: &dw.ContainerComponent{
									Container: dw.Container{
										Image: "test-image",
									},
								},
							},
						},
					},
				},
			},
			Expected: true,
		},
		{
			Name: "Workspace with overrides in template and container component",
			Input: dw.DevWorkspaceTemplateSpec{
				DevWorkspaceTemplateSpecContent: dw.DevWorkspaceTemplateSpecContent{
					Attributes: attributes.Attributes{
						constants.PodOverridesAttribute: jsonPodOverrides,
					},
					Components: []dw.Component{
						{
							Name: "test-component",
							Attributes: attributes.Attributes{
								constants.PodOverridesAttribute: jsonPodOverrides,
							},
							ComponentUnion: dw.ComponentUnion{
								Container: &dw.ContainerComponent{
									Container: dw.Container{
										Image: "test-image",
									},
								},
							},
						},
					},
				},
			},
			Expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			workspace := &common.DevWorkspaceWithConfig{}
			workspace.DevWorkspace = &dw.DevWorkspace{}
			workspace.Spec.Template = tt.Input
			actual := NeedsPodOverrides(workspace)
			assert.Equal(t, tt.Expected, actual)
		})
	}
}

type podTestCase struct {
	Name             string         `json:"name,omitempty"`
	Input            *podTestInput  `json:"input,omitempty"`
	Output           *podTestOutput `json:"output,omitempty"`
	originalFilename string
}

type podTestInput struct {
	Workspace       *dw.DevWorkspaceTemplateSpec `json:"workspace,omitempty"`
	PodTemplateSpec *corev1.PodTemplateSpec      `json:"podTemplateSpec,omitempty"`
}

type podTestOutput struct {
	PodTemplateSpec *corev1.PodTemplateSpec `json:"podTemplateSpec,omitempty"`
	ErrRegexp       *string                 `json:"errRegexp,omitempty"`
}

func loadAllPodTestCasesOrPanic(t *testing.T, fromDir string) []podTestCase {
	files, err := os.ReadDir(fromDir)
	if err != nil {
		t.Fatal(err)
	}
	var tests []podTestCase
	for _, file := range files {
		if file.IsDir() {
			tests = append(tests, loadAllPodTestCasesOrPanic(t, filepath.Join(fromDir, file.Name()))...)
		} else {
			tests = append(tests, loadPodTestCaseOrPanic(t, filepath.Join(fromDir, file.Name())))
		}
	}
	return tests
}

func loadPodTestCaseOrPanic(t *testing.T, testPath string) podTestCase {
	bytes, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatal(err)
	}
	var test podTestCase
	if err := yaml.Unmarshal(bytes, &test); err != nil {
		t.Fatal(err)
	}
	test.originalFilename = testPath
	return test
}
