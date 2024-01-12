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
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func TestApplyContainerOverrides(t *testing.T) {
	tests := loadAllContainerTestCasesOrPanic(t, "testdata/container-overrides")
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s (%s)", tt.Name, tt.originalFilename), func(t *testing.T) {
			outContainer, err := ApplyContainerOverrides(tt.Input.Component, tt.Input.Container)
			if tt.Output.ErrRegexp != nil && assert.Error(t, err) {
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error(), "Error message should match")
			} else {
				if !assert.NoError(t, err, "Should not return error") {
					return
				}
				assert.Truef(t, cmp.Equal(tt.Output.Container, outContainer),
					"Container should match expected output:\n%s",
					cmp.Diff(tt.Output.Container, outContainer))
			}
		})
	}
}

type containerTestCase struct {
	Name             string               `json:"name,omitempty"`
	Input            *containerTestInput  `json:"input,omitempty"`
	Output           *containerTestOutput `json:"output,omitempty"`
	originalFilename string
}

type containerTestInput struct {
	Component *dw.Component     `json:"component,omitempty"`
	Container *corev1.Container `json:"container,omitempty"`
}

type containerTestOutput struct {
	Container *corev1.Container `json:"container,omitempty"`
	ErrRegexp *string           `json:"errRegexp,omitempty"`
}

func loadAllContainerTestCasesOrPanic(t *testing.T, fromDir string) []containerTestCase {
	files, err := os.ReadDir(fromDir)
	if err != nil {
		t.Fatal(err)
	}
	var tests []containerTestCase
	for _, file := range files {
		if file.IsDir() {
			tests = append(tests, loadAllContainerTestCasesOrPanic(t, filepath.Join(fromDir, file.Name()))...)
		} else {
			tests = append(tests, loadContainerTestCaseOrPanic(t, filepath.Join(fromDir, file.Name())))
		}
	}
	return tests
}

func loadContainerTestCaseOrPanic(t *testing.T, testPath string) containerTestCase {
	bytes, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatal(err)
	}
	var test containerTestCase
	if err := yaml.Unmarshal(bytes, &test); err != nil {
		t.Fatal(err)
	}
	test.originalFilename = testPath
	return test
}
