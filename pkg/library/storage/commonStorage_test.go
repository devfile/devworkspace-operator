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

package storage

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/devfile/devworkspace-operator/pkg/config"
	corev1 "k8s.io/api/core/v1"

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
)

type testCase struct {
	Name   string     `json:"name,omitempty"`
	Input  testInput  `json:"input,omitempty"`
	Output testOutput `json:"output,omitempty"`
}

type testInput struct {
	WorkspaceID  string                                `json:"workspaceId,omitempty"`
	PodAdditions v1alpha1.PodAdditions                 `json:"podAdditions,omitempty"`
	Workspace    devworkspace.DevWorkspaceTemplateSpec `json:"workspace,omitempty"`
}

type testOutput struct {
	PodAdditions v1alpha1.PodAdditions `json:"podAdditions,omitempty"`
	ErrRegexp    *string               `json:"errRegexp,omitempty"`
}

var testControllerCfg = &corev1.ConfigMap{
	Data: map[string]string{
		"devworkspace.sidecar.image_pull_policy": "Always",
	},
}

func setupControllerCfg() {
	config.SetupConfigForTesting(testControllerCfg)
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

func TestRewriteContainerVolumeMounts(t *testing.T) {
	tests := []testCase{
		loadTestCaseOrPanic(t, "does-nothing-for-no-storage-needed.yaml"),
		loadTestCaseOrPanic(t, "projects-volume-overriding.yaml"),
		loadTestCaseOrPanic(t, "rewrites-volumes-for-common-pvc-strategy.yaml"),
		loadTestCaseOrPanic(t, "error-duplicate-volumes.yaml"),
		loadTestCaseOrPanic(t, "error-undefined-volume.yaml"),
		loadTestCaseOrPanic(t, "error-undefined-volume-init-container.yaml"),
	}
	setupControllerCfg()

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// sanity check that file is read correctly.
			assert.NotNil(t, tt.Input.Workspace, "Input does not define workspace")
			err := RewriteContainerVolumeMounts(tt.Input.WorkspaceID, &tt.Input.PodAdditions, tt.Input.Workspace)
			if tt.Output.ErrRegexp != nil && assert.Error(t, err) {
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error(), "Error message should match")
			} else {
				if !assert.NoError(t, err, "Should not return error") {
					return
				}
				assert.Equal(t, tt.Output.PodAdditions, tt.Input.PodAdditions, "PodAdditions should match expected output")
			}
		})
	}
}

func TestNeedsStorage(t *testing.T) {
	boolFalse := false
	boolTrue := true
	tests := []struct {
		Name        string
		Explanation string
		Components  []devworkspace.ComponentUnion
	}{
		{
			Name:        "Has volume component",
			Explanation: "If the devfile has a volume component, it requires storage",
			Components: []devworkspace.ComponentUnion{
				{
					Volume: &devworkspace.VolumeComponent{},
				},
			},
		},
		{
			Name:        "Has container component with volume mounts",
			Explanation: "If a devfile container has volumeMounts, it requires storage",
			Components: []devworkspace.ComponentUnion{
				{
					Container: &devworkspace.ContainerComponent{
						Container: devworkspace.Container{
							MountSources: &boolFalse,
							VolumeMounts: []devworkspace.VolumeMount{
								{
									Name: "test-volumeMount",
								},
							},
						},
					},
				},
			},
		},
		{
			Name:        "Container has mountSources",
			Explanation: "If a devfile container has mountSources set, it requires storage",
			Components: []devworkspace.ComponentUnion{
				{
					Container: &devworkspace.ContainerComponent{
						Container: devworkspace.Container{
							MountSources: &boolTrue,
						},
					},
				},
			},
		},
		{
			Name:        "Container has implicit mountSources",
			Explanation: "If a devfile container does not have mountSources set, the default is true",
			Components: []devworkspace.ComponentUnion{
				{
					Container: &devworkspace.ContainerComponent{
						Container: devworkspace.Container{},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			workspace := devworkspace.DevWorkspaceTemplateSpec{}
			for idx, comp := range tt.Components {
				workspace.Components = append(workspace.Components, devworkspace.Component{
					Name:           fmt.Sprintf("test-component-%d", idx),
					ComponentUnion: comp,
				})
			}
			assert.True(t, NeedsStorage(workspace), tt.Explanation)
		})
	}
}
