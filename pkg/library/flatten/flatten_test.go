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

package flatten

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha2"
)

var workspaceTemplateDiffOpts = cmp.Options{
	cmpopts.SortSlices(func(a, b devworkspace.Component) bool {
		return strings.Compare(a.Key(), b.Key()) > 0
	}),
	cmpopts.SortSlices(func(a, b string) bool {
		return strings.Compare(a, b) > 0
	}),
	// TODO: Devworkspace overriding results in empty []string instead of nil
	cmpopts.IgnoreFields(devworkspace.WorkspaceEvents{}, "PostStart", "PreStop", "PostStop"),
}

type testCase struct {
	Name   string     `json:"name"`
	Input  testInput  `json:"input"`
	Output testOutput `json:"output"`
}

type testInput struct {
	Workspace devworkspace.DevWorkspaceTemplateSpec `json:"workspace,omitempty"`
	// Plugins is a map of plugin "name" to devworkspace template; namespace is ignored.
	Plugins map[string]devworkspace.DevWorkspaceTemplate `json:"plugins,omitempty"`
	// Errors is a map of plugin name to the error that should be returned when attempting to retrieve it.
	Errors map[string]testPluginError `json:"errors,omitempty"`
}

type testPluginError struct {
	IsNotFound bool   `json:"isNotFound"`
	Message    string `json:"message"`
}

type testOutput struct {
	Workspace *devworkspace.DevWorkspaceTemplateSpec `json:"workspace,omitempty"`
	ErrRegexp *string                                `json:"errRegexp,omitempty"`
}

type fakeK8sClient struct {
	client.Client // To satisfy interface; override all used methods
	plugins       map[string]devworkspace.DevWorkspaceTemplate
	errors        map[string]testPluginError
}

func (client *fakeK8sClient) Get(_ context.Context, namespacedName client.ObjectKey, obj runtime.Object) error {
	template, ok := obj.(*devworkspace.DevWorkspaceTemplate)
	if !ok {
		return fmt.Errorf("Called Get() in fake client with non-DevWorkspaceTemplate")
	}
	if plugin, ok := client.plugins[namespacedName.Name]; ok {
		*template = plugin
		return nil
	}
	if err, ok := client.errors[namespacedName.Name]; ok {
		if err.IsNotFound {
			return k8sErrors.NewNotFound(schema.GroupResource{}, namespacedName.Name)
		} else {
			return errors.New(err.Message)
		}
	}
	return fmt.Errorf("test does not define an entry for plugin %s", namespacedName.Name)
}

func loadTestCaseOrPanic(t *testing.T, testFilename string) testCase {
	testPath := filepath.Join("./testdata", testFilename)
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

func loadAllTestsOrPanic(t *testing.T) []testCase {
	files, err := ioutil.ReadDir("./testdata")
	if err != nil {
		t.Fatal(err)
	}
	var tests []testCase
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		tests = append(tests, loadTestCaseOrPanic(t, file.Name()))
	}
	return tests
}

func TestResolveDevWorkspace(t *testing.T) {
	tests := loadAllTestsOrPanic(t)
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// sanity check: input defines components
			assert.True(t, len(tt.Input.Workspace.Components) > 0, "Test case defines workspace with no components")
			testClient := &fakeK8sClient{
				plugins: tt.Input.Plugins,
				errors:  tt.Input.Errors,
			}
			testResolverTools := ResolverTools{
				Context:   context.Background(),
				K8sClient: testClient,
			}
			outputWorkspace, err := ResolveDevWorkspace(tt.Input.Workspace, testResolverTools)
			if tt.Output.ErrRegexp != nil && assert.Error(t, err) {
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error(), "Error message should match")
			} else {
				if !assert.NoError(t, err, "Should not return error") {
					return
				}
				assert.Truef(t, cmp.Equal(tt.Output.Workspace, outputWorkspace, workspaceTemplateDiffOpts),
					"Workspace should match expected output:\n%s",
					cmp.Diff(tt.Output.Workspace, outputWorkspace, workspaceTemplateDiffOpts))
			}
		})
	}
}
