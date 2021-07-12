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
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/workspace/provision"
	"github.com/devfile/devworkspace-operator/pkg/config"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/yaml"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(dw.AddToScheme(scheme))
}

type testCase struct {
	Name   string     `json:"name,omitempty"`
	Input  testInput  `json:"input,omitempty"`
	Output testOutput `json:"output,omitempty"`
}

type testInput struct {
	DevWorkspaceID string                       `json:"devworkspaceId,omitempty"`
	PodAdditions   v1alpha1.PodAdditions        `json:"podAdditions,omitempty"`
	Workspace      *dw.DevWorkspaceTemplateSpec `json:"workspace,omitempty"`
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

func loadTestCaseOrPanic(t *testing.T, testFilepath string) testCase {
	bytes, err := ioutil.ReadFile(testFilepath)
	if err != nil {
		t.Fatal(err)
	}
	var test testCase
	if err := yaml.Unmarshal(bytes, &test); err != nil {
		t.Fatal(err)
	}
	return test
}

func loadAllTestCasesOrPanic(t *testing.T, fromDir string) []testCase {
	files, err := ioutil.ReadDir(fromDir)
	if err != nil {
		t.Fatal(err)
	}
	var tests []testCase
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		tests = append(tests, loadTestCaseOrPanic(t, filepath.Join(fromDir, file.Name())))
	}
	return tests
}

func TestRewriteContainerVolumeMountsForCommonStorageClass(t *testing.T) {
	tests := loadAllTestCasesOrPanic(t, "testdata/common-storage")
	setupControllerCfg()
	commonStorage := CommonStorageProvisioner{}
	commonPVC, err := getCommonPVCSpec("test-namespace", "1Gi")
	if err != nil {
		t.Fatalf("Failure during setup: %s", err)
	}
	commonPVC.Status.Phase = corev1.ClaimBound
	clusterAPI := provision.ClusterAPI{
		Client: fake.NewFakeClientWithScheme(scheme, commonPVC),
		Logger: zap.New(),
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// sanity check that file is read correctly.
			assert.NotNil(t, tt.Input.Workspace, "Input does not define workspace")
			workspace := &dw.DevWorkspace{}
			workspace.Spec.Template = *tt.Input.Workspace
			workspace.Status.DevWorkspaceId = tt.Input.DevWorkspaceID
			workspace.Namespace = "test-namespace"
			err := commonStorage.ProvisionStorage(&tt.Input.PodAdditions, workspace, clusterAPI)
			if tt.Output.ErrRegexp != nil && assert.Error(t, err) {
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error(), "Error message should match")
			} else {
				if !assert.NoError(t, err, "Should not return error") {
					return
				}
				sortVolumesAndVolumeMounts(&tt.Output.PodAdditions)
				sortVolumesAndVolumeMounts(&tt.Input.PodAdditions)
				assert.Equal(t, tt.Output.PodAdditions, tt.Input.PodAdditions, "PodAdditions should match expected output")
			}
		})
	}
}

func TestNeedsStorage(t *testing.T) {
	boolTrue := true
	tests := []struct {
		Name         string
		Explanation  string
		NeedsStorage bool
		Components   []dw.Component
	}{
		{
			Name:         "Has volume component",
			Explanation:  "If the devfile has a volume component, it requires storage",
			NeedsStorage: true,
			Components: []dw.Component{
				{
					ComponentUnion: dw.ComponentUnion{
						Volume: &dw.VolumeComponent{},
					},
				},
			},
		},
		{
			Name:         "Has ephemeral volume and does not need storage",
			Explanation:  "Volumes with ephemeral: true do not require storage",
			NeedsStorage: false,
			Components: []dw.Component{
				{
					ComponentUnion: dw.ComponentUnion{
						Volume: &dw.VolumeComponent{
							Volume: dw.Volume{
								Ephemeral: true,
							},
						},
					},
				},
			},
		},
		{
			Name:         "Container has mountSources",
			Explanation:  "If a devfile container has mountSources set, it requires storage",
			NeedsStorage: true,
			Components: []dw.Component{
				{
					ComponentUnion: dw.ComponentUnion{
						Container: &dw.ContainerComponent{
							Container: dw.Container{
								MountSources: &boolTrue,
							},
						},
					},
				},
			},
		},
		{
			Name:         "Container has mountSources but projects is ephemeral",
			Explanation:  "When a devfile has an explicit, ephemeral projects volume, containers with mountSources do not need storage",
			NeedsStorage: false,
			Components: []dw.Component{
				{
					ComponentUnion: dw.ComponentUnion{
						Container: &dw.ContainerComponent{
							Container: dw.Container{
								MountSources: &boolTrue,
							},
						},
					},
				},
				{
					Name: "projects",
					ComponentUnion: dw.ComponentUnion{
						Volume: &dw.VolumeComponent{
							Volume: dw.Volume{
								Ephemeral: true,
							},
						},
					},
				},
			},
		},
		{
			Name:         "Container has implicit mountSources",
			Explanation:  "If a devfile container does not have mountSources set, the default is true",
			NeedsStorage: true,
			Components: []dw.Component{
				{
					ComponentUnion: dw.ComponentUnion{
						Container: &dw.ContainerComponent{
							Container: dw.Container{},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			workspace := &dw.DevWorkspaceTemplateSpec{}
			workspace.Components = tt.Components
			if tt.NeedsStorage {
				assert.True(t, needsStorage(workspace), tt.Explanation)
			} else {
				assert.False(t, needsStorage(workspace), tt.Explanation)
			}
		})
	}
}

func sortVolumesAndVolumeMounts(podAdditions *v1alpha1.PodAdditions) {
	if podAdditions.Volumes != nil {
		sort.Slice(podAdditions.Volumes, func(i, j int) bool {
			return strings.Compare(podAdditions.Volumes[i].Name, podAdditions.Volumes[j].Name) < 0
		})
	}
	for idx, container := range podAdditions.Containers {
		if container.VolumeMounts != nil {
			sort.Slice(podAdditions.Containers[idx].VolumeMounts, func(i, j int) bool {
				return strings.Compare(podAdditions.Containers[idx].VolumeMounts[i].Name, podAdditions.Containers[idx].VolumeMounts[j].Name) < 0
			})
		}
	}
}
