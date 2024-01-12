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

package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

type preStopTestCase struct {
	Name     string            `json:"name,omitempty"`
	Input    preStopTestInput  `json:"input,omitempty"`
	Output   preStopTestOutput `json:"output,omitempty"`
	testPath string
}

type preStopTestInput struct {
	Devfile    *dw.DevWorkspaceTemplateSpec `json:"devfile,omitempty"`
	Containers []corev1.Container           `json:"containers,omitempty"`
}

type preStopTestOutput struct {
	Containers []corev1.Container `json:"containers,omitempty"`
	ErrRegexp  *string            `json:"errRegexp,omitempty"`
}

func loadPreStopTestCaseOrPanic(t *testing.T, testPath string) preStopTestCase {
	bytes, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatal(err)
	}
	var test preStopTestCase
	if err := yaml.Unmarshal(bytes, &test); err != nil {
		t.Fatal(err)
	}
	test.testPath = testPath
	return test
}

func loadAllPreStopTestCasesOrPanic(t *testing.T, fromDir string) []preStopTestCase {
	files, err := os.ReadDir(fromDir)
	if err != nil {
		t.Fatal(err)
	}
	var tests []preStopTestCase
	for _, file := range files {
		if file.IsDir() {
			tests = append(tests, loadAllPreStopTestCasesOrPanic(t, filepath.Join(fromDir, file.Name()))...)
		} else {
			tests = append(tests, loadPreStopTestCaseOrPanic(t, filepath.Join(fromDir, file.Name())))
		}
	}
	return tests
}

func TestAddPreStopLifecycleHooks(t *testing.T) {
	tests := loadAllPreStopTestCasesOrPanic(t, "./testdata/preStop")
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s (%s)", tt.Name, tt.testPath), func(t *testing.T) {
			err := AddPreStopLifecycleHooks(tt.Input.Devfile, tt.Input.Containers)
			if tt.Output.ErrRegexp != nil && assert.Error(t, err) {
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error(), "Error message should match")
			} else {
				if !assert.NoError(t, err, "Should not return error") {
					return
				}
				assert.Equal(t, tt.Output.Containers, tt.Input.Containers, "Containers should be updated to match expected output")
			}
		})
	}
}
