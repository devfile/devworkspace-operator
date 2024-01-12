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
	"os"
	"path/filepath"
	"testing"

	dwv2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
)

type testCase struct {
	Name   string     `json:"name,omitempty"`
	Input  testInput  `json:"input,omitempty"`
	Output testOutput `json:"output,omitempty"`
}

type testInput struct {
	OldWorkspace *dwv2.DevWorkspaceTemplateSpec `json:"oldWorkspace,omitempty"`
	NewWorkspace *dwv2.DevWorkspaceTemplateSpec `json:"newWorkspace,omitempty"`
}

type testOutput struct {
	ExpectedWarning    *string `json:"expectedWarning,omitempty"`
	NewWarningsPresent *bool   `json:"newWarningsPresent,omitempty"`
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

func TestModifyWorkspaceWithUnsupportedFeatures(t *testing.T) {
	tests := loadAllTestCasesOrPanic(t, "testdata/warning")

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// sanity check that file is read correctly.
			assert.NotNil(t, tt.Input.OldWorkspace, "Input does not define an old workspace")
			assert.NotNil(t, tt.Input.NewWorkspace, "Input does not define a new workspace")
			oldWorkspace := &dwv2.DevWorkspace{}
			oldWorkspace.Spec.Template = *tt.Input.OldWorkspace

			newWorkspace := &dwv2.DevWorkspace{}
			newWorkspace.Spec.Template = *tt.Input.NewWorkspace

			warnings := checkForAddedUnsupportedFeatures(oldWorkspace, newWorkspace)
			if tt.Output.NewWarningsPresent != nil && *tt.Output.NewWarningsPresent {
				assert.True(t, unsupportedWarningsPresent(warnings), "New warnings expected")
			} else {
				assert.False(t, unsupportedWarningsPresent(warnings), "No new warnings expected")
			}
			if tt.Output.ExpectedWarning != nil {
				assert.Equal(t, *tt.Output.ExpectedWarning, formatUnsupportedFeaturesWarning(warnings), "Warning message should match")
			}
		})
	}
}
