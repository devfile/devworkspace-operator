//
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
//

package container

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config"
)

type testCase struct {
	Name   string                       `json:"name,omitempty"`
	Input  *dw.DevWorkspaceTemplateSpec `json:"input,omitempty"`
	Output testOutput                   `json:"output,omitempty"`
}

type testOutput struct {
	PodAdditions *v1alpha1.PodAdditions `json:"podAdditions,omitempty"`
	ErrRegexp    *string                `json:"errRegexp,omitempty"`
}

var testControllerCfg = &v1alpha1.OperatorConfiguration{
	Workspace: &v1alpha1.WorkspaceConfig{
		ImagePullPolicy: "Always",
	},
}

func setupControllerCfg() {
	config.SetConfigForTesting(testControllerCfg)
}

func loadAllTestCasesOrPanic(t *testing.T, fromDir string) []testCase {
	files, err := os.ReadDir(fromDir)
	if err != nil {
		t.Fatal(err)
	}
	var tests []testCase
	for _, file := range files {
		if file.IsDir() {
			tests = append(tests, loadAllTestCasesOrPanic(t, filepath.Join(fromDir, file.Name()))...)
		} else {
			tests = append(tests, loadTestCaseOrPanic(t, filepath.Join(fromDir, file.Name())))
		}
	}
	return tests
}

func loadTestCaseOrPanic(t *testing.T, testPath string) testCase {
	bytes, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatal(err)
	}
	var test testCase
	if err := yaml.Unmarshal(bytes, &test); err != nil {
		t.Fatal(err)
	}
	t.Log(fmt.Sprintf("Read file:\n%+v\n\n", test))
	return test
}

func TestGetKubeContainersFromDevfile(t *testing.T) {
	tests := loadAllTestCasesOrPanic(t, "./testdata")
	setupControllerCfg()

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// sanity check that file is read correctly.
			assert.True(t, len(tt.Input.Components) > 0, "Input defines no components")
			gotPodAdditions, err := GetKubeContainersFromDevfile(tt.Input, *testControllerCfg)
			if tt.Output.ErrRegexp != nil && assert.Error(t, err) {
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error(), "Error message should match")
			} else {
				if !assert.NoError(t, err, "Should not return error") {
					return
				}
				assert.True(t, cmp.Equal(tt.Output.PodAdditions, gotPodAdditions),
					"PodAdditions should match expected output: \n%s", cmp.Diff(tt.Output.PodAdditions, gotPodAdditions))
			}
		})
	}
}
