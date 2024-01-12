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

package env

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
)

func TestResolveDevWorkspaceWorkspaceEnv(t *testing.T) {
	tests := loadAllTestsOrPanic(t, "testdata/workspace-env")
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// sanity check: input defines components
			assert.True(t, len(tt.Input.DevWorkspace.Components) > 0, "Test case defines devworkspace with no components")

			envvars, err := collectWorkspaceEnv(tt.Input.DevWorkspace)
			if tt.Output.ErrRegexp != nil && assert.Error(t, err) {
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error(), "Error message should match")
			} else {
				if !assert.NoError(t, err, "Should not return error") {
					return
				}
				assert.ElementsMatch(t, tt.Output.WorkspaceEnv, envvars, "Workspace env vars should match")
			}
		})
	}
}

type TestCase struct {
	Name   string     `json:"name"`
	Input  TestInput  `json:"input"`
	Output TestOutput `json:"output"`
}

type TestInput struct {
	DevWorkspace *dw.DevWorkspaceTemplateSpec `json:"devworkspace,omitempty"`
}

type TestOutput struct {
	WorkspaceEnv []corev1.EnvVar `json:"workspaceEnv,omitempty"`
	ErrRegexp    *string         `json:"errRegexp,omitempty"`
}

func loadTestCaseOrPanic(t *testing.T, testFilepath string) TestCase {
	bytes, err := ioutil.ReadFile(testFilepath)
	if err != nil {
		t.Fatal(err)
	}
	var test TestCase
	if err := yaml.Unmarshal(bytes, &test); err != nil {
		t.Fatal(err)
	}
	return test
}

func loadAllTestsOrPanic(t *testing.T, fromDir string) []TestCase {
	files, err := ioutil.ReadDir(fromDir)
	if err != nil {
		t.Fatal(err)
	}
	var tests []TestCase
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		tests = append(tests, loadTestCaseOrPanic(t, filepath.Join(fromDir, file.Name())))
	}
	return tests
}
