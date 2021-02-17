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
			assert.True(t, len(tt.Input.Workspace.Components) > 0, "Test case defines workspace with no components")
			testClient := &testutil.FakeK8sClient{
				Plugins: tt.Input.Plugins,
				Errors:  tt.Input.Errors,
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
				assert.Truef(t, cmp.Equal(tt.Output.Workspace, outputWorkspace, testutil.WorkspaceTemplateDiffOpts),
					"Workspace should match expected output:\n%s",
					cmp.Diff(tt.Output.Workspace, outputWorkspace, testutil.WorkspaceTemplateDiffOpts))
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
			assert.True(t, len(tt.Input.Workspace.Components) > 0, "Test case defines workspace with no components")
			testRegistry := &testutil.FakeInternalRegistry{
				Plugins: tt.Input.Plugins,
				Errors:  tt.Input.Errors,
			}
			testResolverTools := ResolverTools{
				Context:          context.Background(),
				InternalRegistry: testRegistry,
			}
			outputWorkspace, err := ResolveDevWorkspace(tt.Input.Workspace, testResolverTools)
			if tt.Output.ErrRegexp != nil && assert.Error(t, err) {
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error(), "Error message should match")
			} else {
				if !assert.NoError(t, err, "Should not return error") {
					return
				}
				assert.Truef(t, cmp.Equal(tt.Output.Workspace, outputWorkspace, testutil.WorkspaceTemplateDiffOpts),
					"Workspace should match expected output:\n%s",
					cmp.Diff(tt.Output.Workspace, outputWorkspace, testutil.WorkspaceTemplateDiffOpts))
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
			assert.True(t, len(tt.Input.Workspace.Components) > 0, "Test case defines workspace with no components")
			testHttpGetter := &testutil.FakeHTTPGetter{
				Plugins: tt.Input.DevfilePlugins,
				Errors:  tt.Input.Errors,
			}
			testResolverTools := ResolverTools{
				Context:    context.Background(),
				HttpClient: testHttpGetter,
			}
			outputWorkspace, err := ResolveDevWorkspace(tt.Input.Workspace, testResolverTools)
			if tt.Output.ErrRegexp != nil && assert.Error(t, err) {
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error(), "Error message should match")
			} else {
				if !assert.NoError(t, err, "Should not return error") {
					return
				}
				assert.Truef(t, cmp.Equal(tt.Output.Workspace, outputWorkspace, testutil.WorkspaceTemplateDiffOpts),
					"Workspace should match expected output:\n%s",
					cmp.Diff(tt.Output.Workspace, outputWorkspace, testutil.WorkspaceTemplateDiffOpts))
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
			assert.True(t, len(tt.Input.Workspace.Components) > 0, "Test case defines workspace with no components")
			testHttpGetter := &testutil.FakeHTTPGetter{
				Plugins: tt.Input.DevfilePlugins,
				Errors:  tt.Input.Errors,
			}
			testResolverTools := ResolverTools{
				Context:    context.Background(),
				HttpClient: testHttpGetter,
			}
			outputWorkspace, err := ResolveDevWorkspace(tt.Input.Workspace, testResolverTools)
			if tt.Output.ErrRegexp != nil && assert.Error(t, err) {
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error(), "Error message should match")
			} else {
				if !assert.NoError(t, err, "Should not return error") {
					return
				}
				assert.Truef(t, cmp.Equal(tt.Output.Workspace, outputWorkspace, testutil.WorkspaceTemplateDiffOpts),
					"Workspace should match expected output:\n%s",
					cmp.Diff(tt.Output.Workspace, outputWorkspace, testutil.WorkspaceTemplateDiffOpts))
			}
		})
	}
}
