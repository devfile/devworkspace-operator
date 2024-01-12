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

package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"sigs.k8s.io/yaml"
)

var WorkspaceTemplateDiffOpts = cmp.Options{
	cmpopts.SortSlices(func(a, b dw.Component) bool {
		return strings.Compare(a.Key(), b.Key()) > 0
	}),
	cmpopts.SortSlices(func(a, b string) bool {
		return strings.Compare(a, b) > 0
	}),
	cmpopts.SortSlices(func(a, b dw.EnvVar) bool {
		return strings.Compare(a.Name, b.Name) > 0
	}),
	cmpopts.SortSlices(func(a, b dw.Endpoint) bool {
		return strings.Compare(a.Name, b.Name) > 0
	}),
	cmpopts.SortSlices(func(a, b dw.VolumeMount) bool {
		return strings.Compare(a.Name, b.Name) > 0
	}),
	cmpopts.SortSlices(func(a, b dw.Command) bool {
		return strings.Compare(a.Key(), b.Key()) > 0
	}),
	// TODO: Devworkspace overriding results in empty []string instead of nil
	cmpopts.IgnoreFields(dw.DevWorkspaceEvents{}, "PostStart", "PreStop", "PostStop"),
}

type TestCase struct {
	Name     string     `json:"name"`
	Input    TestInput  `json:"input"`
	Output   TestOutput `json:"output"`
	TestPath string
}

type TestInput struct {
	// DevWorkspace is the .spec.template field of a DevWorkspace
	DevWorkspace *dw.DevWorkspaceTemplateSpec `json:"devworkspace,omitempty"`
	// Contributions is the .spec.containerContributions field of a DevWorkspace
	Contributions []dw.ComponentContribution `json:"contributions,omitempty"`
	// DevWorkspaceResources is a map of string keys to devworkspace templates
	DevWorkspaceResources map[string]dw.DevWorkspaceTemplate `json:"devworkspaceResources,omitempty"`
	// DevfileResources is a map of string keys to devfile resources
	DevfileResources map[string]dw.Devfile `json:"devfileResources,omitempty"`
	// Errors is a map of plugin name to the error that should be returned when attempting to retrieve it.
	Errors map[string]TestPluginError `json:"errors,omitempty"`
}

type TestPluginError struct {
	// IsNotFound marks this error as a kubernetes NotFoundError
	IsNotFound bool `json:"isNotFound"`
	// StatusCode defines the HTTP response code (if relevant)
	StatusCode int `json:"statusCode"`
	// Message is the error message returned
	Message string `json:"message"`
}

type TestOutput struct {
	DevWorkspace *dw.DevWorkspaceTemplateSpec `json:"devworkspace,omitempty"`
	ErrRegexp    *string                      `json:"errRegexp,omitempty"`
}

func LoadTestCaseOrPanic(t *testing.T, testFilepath string) TestCase {
	bytes, err := os.ReadFile(testFilepath)
	if err != nil {
		t.Fatal(err)
	}
	var test TestCase
	if err := yaml.Unmarshal(bytes, &test); err != nil {
		t.Fatal(err)
	}
	test.TestPath = testFilepath
	return test
}

func LoadAllTestsOrPanic(t *testing.T, fromDir string) []TestCase {
	files, err := os.ReadDir(fromDir)
	if err != nil {
		t.Fatal(err)
	}
	var tests []TestCase
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		tests = append(tests, LoadTestCaseOrPanic(t, filepath.Join(fromDir, file.Name())))
	}
	return tests
}
