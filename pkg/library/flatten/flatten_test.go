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

package flatten

import (
	"context"
	"testing"

	"github.com/devfile/devworkspace-operator/pkg/library/flatten/internal/testutil"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
)

func TestResolveDevWorkspaceKubernetesReference(t *testing.T) {
	tests := testutil.LoadAllTestsOrPanic(t, "testdata/k8s-ref")
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// sanity check: input defines components
			assert.True(t, len(tt.Input.DevWorkspace.Components) > 0, "Test case defines workspace with no components")
			testClient := &testutil.FakeK8sClient{
				DevWorkspaceResources: tt.Input.DevWorkspaceResources,
				Errors:                tt.Input.Errors,
			}
			testResolverTools := ResolverTools{
				Context:            context.Background(),
				WorkspaceNamespace: "test-ignored",
				K8sClient:          testClient,
			}
			outputWorkspace, _, err := ResolveDevWorkspace(tt.Input.DevWorkspace, testResolverTools)
			if tt.Output.ErrRegexp != nil && assert.Error(t, err) {
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error(), "Error message should match")
			} else {
				if !assert.NoError(t, err, "Should not return error") {
					return
				}
				assert.Truef(t, cmp.Equal(tt.Output.DevWorkspace, outputWorkspace, testutil.WorkspaceTemplateDiffOpts),
					"DevWorkspace should match expected output:\n%s",
					cmp.Diff(tt.Output.DevWorkspace, outputWorkspace, testutil.WorkspaceTemplateDiffOpts))
			}
		})
	}
}

func TestResolveDevWorkspaceInternalRegistry(t *testing.T) {
	tests := testutil.LoadAllTestsOrPanic(t, "testdata/internal-registry")
	testutil.SetupControllerCfg()
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// sanity check: input defines components
			assert.True(t, len(tt.Input.DevWorkspace.Components) > 0, "Test case defines workspace with no components")
			testRegistry := &testutil.FakeInternalRegistry{
				Plugins: tt.Input.DevWorkspaceResources,
				Errors:  tt.Input.Errors,
			}
			testResolverTools := ResolverTools{
				Context:          context.Background(),
				InternalRegistry: testRegistry,
			}
			outputWorkspace, _, err := ResolveDevWorkspace(tt.Input.DevWorkspace, testResolverTools)
			if tt.Output.ErrRegexp != nil && assert.Error(t, err) {
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error(), "Error message should match")
			} else {
				if !assert.NoError(t, err, "Should not return error") {
					return
				}
				assert.Truef(t, cmp.Equal(tt.Output.DevWorkspace, outputWorkspace, testutil.WorkspaceTemplateDiffOpts),
					"DevWorkspace should match expected output:\n%s",
					cmp.Diff(tt.Output.DevWorkspace, outputWorkspace, testutil.WorkspaceTemplateDiffOpts))
			}
		})
	}
}

func TestResolveDevWorkspacePluginRegistry(t *testing.T) {
	tests := testutil.LoadAllTestsOrPanic(t, "testdata/plugin-id")
	testutil.SetupControllerCfg()
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// sanity check: input defines components
			assert.True(t, len(tt.Input.DevWorkspace.Components) > 0, "Test case defines workspace with no components")
			testHttpGetter := &testutil.FakeHTTPGetter{
				DevfileResources:      tt.Input.DevfileResources,
				DevWorkspaceResources: tt.Input.DevWorkspaceResources,
				Errors:                tt.Input.Errors,
			}
			testResolverTools := ResolverTools{
				Context:    context.Background(),
				HttpClient: testHttpGetter,
			}
			outputWorkspace, _, err := ResolveDevWorkspace(tt.Input.DevWorkspace, testResolverTools)
			if tt.Output.ErrRegexp != nil && assert.Error(t, err) {
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error(), "Error message should match")
			} else {
				if !assert.NoError(t, err, "Should not return error") {
					return
				}
				assert.Truef(t, cmp.Equal(tt.Output.DevWorkspace, outputWorkspace, testutil.WorkspaceTemplateDiffOpts),
					"DevWorkspace should match expected output:\n%s",
					cmp.Diff(tt.Output.DevWorkspace, outputWorkspace, testutil.WorkspaceTemplateDiffOpts))
			}
		})
	}
}

func TestResolveDevWorkspacePluginURI(t *testing.T) {
	tests := testutil.LoadAllTestsOrPanic(t, "testdata/plugin-uri")
	testutil.SetupControllerCfg()
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// sanity check: input defines components
			assert.True(t, len(tt.Input.DevWorkspace.Components) > 0, "Test case defines workspace with no components")
			testHttpGetter := &testutil.FakeHTTPGetter{
				DevfileResources:      tt.Input.DevfileResources,
				DevWorkspaceResources: tt.Input.DevWorkspaceResources,
				Errors:                tt.Input.Errors,
			}
			testResolverTools := ResolverTools{
				Context:    context.Background(),
				HttpClient: testHttpGetter,
			}
			outputWorkspace, _, err := ResolveDevWorkspace(tt.Input.DevWorkspace, testResolverTools)
			if tt.Output.ErrRegexp != nil && assert.Error(t, err) {
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error(), "Error message should match")
			} else {
				if !assert.NoError(t, err, "Should not return error") {
					return
				}
				assert.Truef(t, cmp.Equal(tt.Output.DevWorkspace, outputWorkspace, testutil.WorkspaceTemplateDiffOpts),
					"DevWorkspace should match expected output:\n%s",
					cmp.Diff(tt.Output.DevWorkspace, outputWorkspace, testutil.WorkspaceTemplateDiffOpts))
			}
		})
	}
}

func TestResolveDevWorkspaceParents(t *testing.T) {
	tests := testutil.LoadAllTestsOrPanic(t, "testdata/parent")
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// sanity check: input defines components
			assert.True(t, len(tt.Input.DevWorkspace.Components) > 0, "Test case defines workspace with no components")
			testHttpGetter := &testutil.FakeHTTPGetter{
				DevfileResources:      tt.Input.DevfileResources,
				DevWorkspaceResources: tt.Input.DevWorkspaceResources,
				Errors:                tt.Input.Errors,
			}
			testK8sClient := &testutil.FakeK8sClient{
				DevWorkspaceResources: tt.Input.DevWorkspaceResources,
				Errors:                tt.Input.Errors,
			}
			testResolverTools := ResolverTools{
				Context:            context.Background(),
				WorkspaceNamespace: "test-ignored",
				K8sClient:          testK8sClient,
				HttpClient:         testHttpGetter,
			}
			outputWorkspace, _, err := ResolveDevWorkspace(tt.Input.DevWorkspace, testResolverTools)
			if tt.Output.ErrRegexp != nil && assert.Error(t, err) {
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error(), "Error message should match")
			} else {
				if !assert.NoError(t, err, "Should not return error") {
					return
				}
				assert.Truef(t, cmp.Equal(tt.Output.DevWorkspace, outputWorkspace, testutil.WorkspaceTemplateDiffOpts),
					"DevWorkspace should match expected output:\n%s",
					cmp.Diff(tt.Output.DevWorkspace, outputWorkspace, testutil.WorkspaceTemplateDiffOpts))
			}
		})
	}
}

func TestResolveDevWorkspaceMissingDefaults(t *testing.T) {
	tests := []testutil.TestCase{
		testutil.LoadTestCaseOrPanic(t, "testdata/general/fail-nicely-when-no-registry-url.yaml"),
		testutil.LoadTestCaseOrPanic(t, "testdata/general/fail-nicely-when-no-namespace.yaml"),
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// sanity check: input defines components
			assert.True(t, len(tt.Input.DevWorkspace.Components) > 0, "Test case defines workspace with no components")
			testHttpGetter := &testutil.FakeHTTPGetter{
				DevfileResources:      tt.Input.DevfileResources,
				DevWorkspaceResources: tt.Input.DevWorkspaceResources,
				Errors:                tt.Input.Errors,
			}
			testK8sClient := &testutil.FakeK8sClient{
				DevWorkspaceResources: tt.Input.DevWorkspaceResources,
				Errors:                tt.Input.Errors,
			}
			testResolverTools := ResolverTools{
				Context:    context.Background(),
				K8sClient:  testK8sClient,
				HttpClient: testHttpGetter,
			}
			outputWorkspace, _, err := ResolveDevWorkspace(tt.Input.DevWorkspace, testResolverTools)
			if tt.Output.ErrRegexp != nil && assert.Error(t, err) {
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error(), "Error message should match")
			} else {
				if !assert.NoError(t, err, "Should not return error") {
					return
				}
				assert.Truef(t, cmp.Equal(tt.Output.DevWorkspace, outputWorkspace, testutil.WorkspaceTemplateDiffOpts),
					"DevWorkspace should match expected output:\n%s",
					cmp.Diff(tt.Output.DevWorkspace, outputWorkspace, testutil.WorkspaceTemplateDiffOpts))
			}
		})
	}
}

func TestResolveDevWorkspaceAnnotations(t *testing.T) {
	tests := testutil.LoadAllTestsOrPanic(t, "testdata/annotate")
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// sanity check: input defines components
			assert.True(t, len(tt.Input.DevWorkspace.Components) > 0, "Test case defines devworkspace with no components")
			testHttpGetter := &testutil.FakeHTTPGetter{
				DevfileResources:      tt.Input.DevfileResources,
				DevWorkspaceResources: tt.Input.DevWorkspaceResources,
				Errors:                tt.Input.Errors,
			}
			testK8sClient := &testutil.FakeK8sClient{
				DevWorkspaceResources: tt.Input.DevWorkspaceResources,
				Errors:                tt.Input.Errors,
			}
			testResolverTools := ResolverTools{
				Context:            context.Background(),
				K8sClient:          testK8sClient,
				HttpClient:         testHttpGetter,
				WorkspaceNamespace: "default-namespace",
			}
			outputWorkspace, _, err := ResolveDevWorkspace(tt.Input.DevWorkspace, testResolverTools)
			if tt.Output.ErrRegexp != nil && assert.Error(t, err) {
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error(), "Error message should match")
			} else {
				if !assert.NoError(t, err, "Should not return error") {
					return
				}
				assert.Truef(t, cmp.Equal(tt.Output.DevWorkspace, outputWorkspace, testutil.WorkspaceTemplateDiffOpts),
					"DevWorkspace should match expected output:\n%s",
					cmp.Diff(tt.Output.DevWorkspace, outputWorkspace, testutil.WorkspaceTemplateDiffOpts))
			}
		})
	}
}

func TestResolveDevWorkspaceTemplateNamespaceRestriction(t *testing.T) {
	tests := testutil.LoadAllTestsOrPanic(t, "testdata/namespace-restriction")
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// sanity check: input defines components
			assert.True(t, len(tt.Input.DevWorkspace.Components) > 0, "Test case defines devworkspace with no components")
			testHttpGetter := &testutil.FakeHTTPGetter{
				DevfileResources:      tt.Input.DevfileResources,
				DevWorkspaceResources: tt.Input.DevWorkspaceResources,
				Errors:                tt.Input.Errors,
			}
			testK8sClient := &testutil.FakeK8sClient{
				DevWorkspaceResources: tt.Input.DevWorkspaceResources,
				Errors:                tt.Input.Errors,
			}
			testResolverTools := ResolverTools{
				Context:            context.Background(),
				K8sClient:          testK8sClient,
				HttpClient:         testHttpGetter,
				WorkspaceNamespace: "test-namespace",
			}
			outputWorkspace, _, err := ResolveDevWorkspace(tt.Input.DevWorkspace, testResolverTools)
			if tt.Output.ErrRegexp != nil && assert.Error(t, err) {
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error(), "Error message should match")
			} else {
				if !assert.NoError(t, err, "Should not return error") {
					return
				}
				assert.Truef(t, cmp.Equal(tt.Output.DevWorkspace, outputWorkspace, testutil.WorkspaceTemplateDiffOpts),
					"DevWorkspace should match expected output:\n%s",
					cmp.Diff(tt.Output.DevWorkspace, outputWorkspace, testutil.WorkspaceTemplateDiffOpts))
			}
		})
	}
}
