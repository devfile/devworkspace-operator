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

package home

import (
	"os"
	"path/filepath"
	"testing"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/assert"
)

type testCase struct {
	Name   string     `json:"name,omitempty"`
	Input  testInput  `json:"input,omitempty"`
	Output testOutput `json:"output,omitempty"`
}

type testInput struct {
	DevWorkspaceID string                          `json:"devworkspaceId,omitempty"`
	Workspace      *dw.DevWorkspaceTemplateSpec    `json:"workspace,omitempty"`
	Config         *v1alpha1.OperatorConfiguration `json:"config,omitempty"`
}

type testOutput struct {
	Workspace *dw.DevWorkspaceTemplateSpec `json:"workspace,omitempty"`
	Error     *string                      `json:"error,omitempty"`
}

func loadTestCaseOrPanic(t *testing.T, testFilepath string) testCase {
	bytes, err := os.ReadFile(testFilepath)
	if err != nil {
		t.Fatal(err)
	}
	var test testCase
	if err := yaml.Unmarshal(bytes, &test); err != nil {
		t.Fatal(err)
	}
	return test
}

func loadAllTestCasesOrPanic(t *testing.T, fromDir string) []testCase {
	files, err := os.ReadDir(fromDir)
	if err != nil {
		t.Fatal(err)
	}
	var tests []testCase
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		tests = append(tests, loadTestCaseOrPanic(t, filepath.Join(fromDir, file.Name())))
	}
	return tests
}

func getDevWorkspaceWithConfig(input testInput) *common.DevWorkspaceWithConfig {
	return &common.DevWorkspaceWithConfig{
		DevWorkspace: &dw.DevWorkspace{
			Spec: dw.DevWorkspaceSpec{
				Template: *input.Workspace,
			},
		},
		Config: input.Config,
	}
}

func TestPersistentHomeVolume(t *testing.T) {
	tests := loadAllTestCasesOrPanic(t, "testdata/persistent-home")
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// sanity check that file is read correctly.
			assert.NotNil(t, tt.Input.Workspace, "Input does not define workspace")
			assert.NotNil(t, tt.Input.Config, "Input does not define a config")
			workspace := getDevWorkspaceWithConfig(tt.Input)
			actualDWTemplateSpec := &workspace.Spec.Template

			if NeedsPersistentHomeDirectory(workspace) {
				workspaceWithHomeVolume, err := AddPersistentHomeVolume(workspace)

				if tt.Output.Error != nil {
					assert.Error(t, err, "Error expected")
					assert.Equal(t, *tt.Output.Error, err.Error())
				} else {
					assert.NoError(t, err)
					workspace.Spec.Template = *workspaceWithHomeVolume
				}
			}

			assert.Equal(t, tt.Output.Workspace, actualDWTemplateSpec,
				"DevWorkspace Template Spec should match expected output: Diff: %s",
				cmp.Diff(tt.Output.Workspace, actualDWTemplateSpec))
		})
	}

}
