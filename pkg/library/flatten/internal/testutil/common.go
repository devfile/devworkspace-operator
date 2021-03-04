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

package testutil

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/devfile/devworkspace-operator/pkg/config"
)

var WorkspaceTemplateDiffOpts = cmp.Options{
	cmpopts.SortSlices(func(a, b dw.Component) bool {
		return strings.Compare(a.Key(), b.Key()) > 0
	}),
	cmpopts.SortSlices(func(a, b string) bool {
		return strings.Compare(a, b) > 0
	}),
	// TODO: Devworkspace overriding results in empty []string instead of nil
	cmpopts.IgnoreFields(dw.DevWorkspaceEvents{}, "PostStart", "PreStop", "PostStop"),
}

var testControllerCfg = &corev1.ConfigMap{
	Data: map[string]string{
		"devworkspace.default_dockerimage.redhat-developer.web-terminal": `
name: default-web-terminal-tooling
container:
  name: default-web-terminal-tooling-container
  image: test-image
`,
	},
}

func SetupControllerCfg() {
	config.SetupConfigForTesting(testControllerCfg)
}

type TestCase struct {
	Name   string     `json:"name"`
	Input  TestInput  `json:"input"`
	Output TestOutput `json:"output"`
}

type TestInput struct {
	DevWorkspace *dw.DevWorkspaceTemplateSpec `json:"devworkspace,omitempty"`
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

func LoadAllTestsOrPanic(t *testing.T, fromDir string) []TestCase {
	files, err := ioutil.ReadDir(fromDir)
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
