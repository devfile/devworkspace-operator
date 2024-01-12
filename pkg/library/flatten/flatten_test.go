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

package flatten

import (
	"context"
	"fmt"
	"testing"

	"github.com/devfile/devworkspace-operator/pkg/library/flatten/internal/testutil"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
)

func TestResolveDevWorkspaceKubernetesReference(t *testing.T) {
	tests := testutil.LoadAllTestsOrPanic(t, "testdata/k8s-ref")
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s (%s)", tt.Name, tt.TestPath), func(t *testing.T) {
			// sanity check: input defines components
			assert.True(t, len(tt.Input.DevWorkspace.Components) > 0, "Test case defines workspace with no components")
			testResolverTools := getTestingTools(tt.Input, "test-ignored")
			outputWorkspace, _, err := ResolveDevWorkspace(tt.Input.DevWorkspace, nil, testResolverTools)
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
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s (%s)", tt.Name, tt.TestPath), func(t *testing.T) {
			// sanity check: input defines components
			assert.True(t, len(tt.Input.DevWorkspace.Components) > 0, "Test case defines workspace with no components")
			testResolverTools := getTestingTools(tt.Input, "test-ignored")

			outputWorkspace, _, err := ResolveDevWorkspace(tt.Input.DevWorkspace, nil, testResolverTools)
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
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s (%s)", tt.Name, tt.TestPath), func(t *testing.T) {
			// sanity check: input defines components
			assert.True(t, len(tt.Input.DevWorkspace.Components) > 0, "Test case defines workspace with no components")
			testResolverTools := getTestingTools(tt.Input, "test-ignored")

			outputWorkspace, _, err := ResolveDevWorkspace(tt.Input.DevWorkspace, nil, testResolverTools)
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
		t.Run(fmt.Sprintf("%s (%s)", tt.Name, tt.TestPath), func(t *testing.T) {
			// sanity check: input defines components
			assert.True(t, len(tt.Input.DevWorkspace.Components) > 0, "Test case defines workspace with no components")
			testResolverTools := getTestingTools(tt.Input, "test-ignored")

			outputWorkspace, _, err := ResolveDevWorkspace(tt.Input.DevWorkspace, nil, testResolverTools)
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
		t.Run(fmt.Sprintf("%s (%s)", tt.Name, tt.TestPath), func(t *testing.T) {
			// sanity check: input defines components
			assert.True(t, len(tt.Input.DevWorkspace.Components) > 0, "Test case defines workspace with no components")
			testResolverTools := getTestingTools(tt.Input, "")

			outputWorkspace, _, err := ResolveDevWorkspace(tt.Input.DevWorkspace, nil, testResolverTools)
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
		t.Run(fmt.Sprintf("%s (%s)", tt.Name, tt.TestPath), func(t *testing.T) {
			// sanity check: input defines components
			assert.True(t, len(tt.Input.DevWorkspace.Components) > 0, "Test case defines devworkspace with no components")
			testResolverTools := getTestingTools(tt.Input, "test-ignored")

			outputWorkspace, _, err := ResolveDevWorkspace(tt.Input.DevWorkspace, nil, testResolverTools)
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
		t.Run(fmt.Sprintf("%s (%s)", tt.Name, tt.TestPath), func(t *testing.T) {
			// sanity check: input defines components
			assert.True(t, len(tt.Input.DevWorkspace.Components) > 0, "Test case defines devworkspace with no components")
			testResolverTools := getTestingTools(tt.Input, "test-namespace")

			outputWorkspace, _, err := ResolveDevWorkspace(tt.Input.DevWorkspace, nil, testResolverTools)
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

func TestMergesDuplicateVolumeComponents(t *testing.T) {
	tests := testutil.LoadAllTestsOrPanic(t, "testdata/volume_merging")
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s (%s)", tt.Name, tt.TestPath), func(t *testing.T) {
			// sanity check: input defines components
			assert.True(t, len(tt.Input.DevWorkspace.Components) > 0, "Test case defines workspace with no components")
			testResolverTools := getTestingTools(tt.Input, "test-ignored")

			outputWorkspace, _, err := ResolveDevWorkspace(tt.Input.DevWorkspace, nil, testResolverTools)
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

func TestMergeContainerContributions(t *testing.T) {
	tests := testutil.LoadAllTestsOrPanic(t, "testdata/container-contributions")
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s (%s)", tt.Name, tt.TestPath), func(t *testing.T) {
			// sanity check: input defines components
			assert.True(t, len(tt.Input.DevWorkspace.Components) > 0, "Test case defines workspace with no components")
			testResolverTools := getTestingTools(tt.Input, "test-ignored")

			outputWorkspace, _, err := ResolveDevWorkspace(tt.Input.DevWorkspace, nil, testResolverTools)
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

func TestMergeImplicitContainerContributions(t *testing.T) {
	tests := testutil.LoadAllTestsOrPanic(t, "testdata/implicit-container-contributions")
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s (%s)", tt.Name, tt.TestPath), func(t *testing.T) {
			// sanity check: input defines components
			assert.True(t, len(tt.Input.DevWorkspace.Components) > 0, "Test case defines workspace with no components")
			testResolverTools := getTestingTools(tt.Input, "test-ignored")

			outputWorkspace, _, err := ResolveDevWorkspace(tt.Input.DevWorkspace, nil, testResolverTools)
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

func TestMergeSpecContributions(t *testing.T) {
	tests := testutil.LoadAllTestsOrPanic(t, "testdata/spec-contributions")
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s (%s)", tt.Name, tt.TestPath), func(t *testing.T) {
			testResolverTools := getTestingTools(tt.Input, "test-namespace")

			outputWorkspace, _, err := ResolveDevWorkspace(tt.Input.DevWorkspace, tt.Input.Contributions, testResolverTools)
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

func TestMergeImplicitSpecContributions(t *testing.T) {
	tests := testutil.LoadAllTestsOrPanic(t, "testdata/implicit-spec-contributions")
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s (%s)", tt.Name, tt.TestPath), func(t *testing.T) {
			testResolverTools := getTestingTools(tt.Input, "test-namespace")

			outputWorkspace, _, err := ResolveDevWorkspace(tt.Input.DevWorkspace, tt.Input.Contributions, testResolverTools)
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

func getTestingTools(input testutil.TestInput, testNamespace string) ResolverTools {
	testHttpGetter := &testutil.FakeHTTPGetter{
		DevfileResources:      input.DevfileResources,
		DevWorkspaceResources: input.DevWorkspaceResources,
		Errors:                input.Errors,
	}
	testK8sClient := &testutil.FakeK8sClient{
		DevWorkspaceResources: input.DevWorkspaceResources,
		Errors:                input.Errors,
	}
	return ResolverTools{
		Context:            context.Background(),
		K8sClient:          testK8sClient,
		HttpClient:         testHttpGetter,
		WorkspaceNamespace: testNamespace,
	}
}
