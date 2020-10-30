//
// Copyright (c) 2019-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package library

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/devfile/api/pkg/apis/workspaces/v1alpha1"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
)

type testCase struct {
	Name   string                                   `json:"name,omitempty"`
	Input  v1alpha1.DevWorkspaceTemplateSpecContent `json:"input,omitempty"`
	Output testOutput                               `json:"output,omitempty"`
}

type testOutput struct {
	InitContainers []v1alpha1.Component `json:"initContainers,omitempty"`
	MainContainers []v1alpha1.Component `json:"mainContainers,omitempty"`
	ErrRegexp      *string              `json:"errRegexp,omitempty"`
}

func loadTestCaseOrPanic(t *testing.T, testFilename string) testCase {
	testPath := filepath.Join("./testdata/lifecycle", testFilename)
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

func TestGetInitContainers(t *testing.T) {
	tests := []testCase{
		loadTestCaseOrPanic(t, "no_events.yaml"),
		loadTestCaseOrPanic(t, "prestart_exec_command.yaml"),
		loadTestCaseOrPanic(t, "prestart_apply_command.yaml"),
		loadTestCaseOrPanic(t, "init_and_main_container.yaml"),
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// sanity check that file reads correctly.
			assert.True(t, len(tt.Input.Components) > 0, "Input defines no components")
			gotInitContainers, gotMainComponents, err := GetInitContainers(tt.Input)
			if tt.Output.ErrRegexp != nil && assert.Error(t, err) {
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error(), "Error message should match")
			} else {
				assert.Equal(t, tt.Output.InitContainers, gotInitContainers, "Init containers should match expected")
				assert.Equal(t, tt.Output.MainContainers, gotMainComponents, "Main containers should match expected")
			}
		})
	}
}
