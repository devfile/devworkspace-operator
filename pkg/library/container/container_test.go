//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package container

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
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

var testControllerCfg = &corev1.ConfigMap{
	Data: map[string]string{
		"devworkspace.sidecar.image_pull_policy": "Always",
	},
}

func setupControllerCfg() {
	config.SetupConfigForTesting(testControllerCfg)
}

func loadAllTestCasesOrPanic(t *testing.T, fromDir string) []testCase {
	files, err := ioutil.ReadDir(fromDir)
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
	bytes, err := ioutil.ReadFile(testPath)
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
			gotPodAdditions, err := GetKubeContainersFromDevfile(tt.Input)
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
