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

package storage

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/yaml"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config"
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
	PVCSize      *resource.Quantity    `json:"pvcSize,omitempty"`
	ErrRegexp    *string               `json:"errRegexp,omitempty"`
}

var testControllerCfg = config.GetConfigForTesting(&v1alpha1.OperatorConfiguration{
	Workspace: &v1alpha1.WorkspaceConfig{
		ImagePullPolicy: "Always",
	},
})

func getDevWorkspaceWithConfig(workspace *dw.DevWorkspace) *common.DevWorkspaceWithConfig {
	workspaceWithConfig := &common.DevWorkspaceWithConfig{}
	workspaceWithConfig.DevWorkspace = workspace
	workspaceWithConfig.Config = testControllerCfg
	return workspaceWithConfig
}

func loadTestCaseOrPanic(t *testing.T, testFilepath string) testCase {
	bytes, err := os.ReadFile(testFilepath)
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
	files, err := os.ReadDir(fromDir)
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

func TestUseCommonStorageProvisionerForPerUserStorageClass(t *testing.T) {
	test := loadTestCaseOrPanic(t, "testdata/common-storage/per-user-alias.yaml")

	t.Run(test.Name, func(t *testing.T) {
		// sanity check that file is read correctly.
		assert.NotNil(t, test.Input.Workspace, "Input does not define workspace")
		workspace := &dw.DevWorkspace{}
		workspace.Spec.Template = *test.Input.Workspace
		storageProvisioner, err := GetProvisioner(getDevWorkspaceWithConfig(workspace))

		if !assert.NoError(t, err, "Should not return error") {
			return
		}
		assert.Equal(t, &CommonStorageProvisioner{}, storageProvisioner, "Per-user storage class should use the common storage provisioner")
	})
}

func TestProvisionStorageForCommonStorageClass(t *testing.T) {
	tests := loadAllTestCasesOrPanic(t, "testdata/common-storage")
	commonStorage := CommonStorageProvisioner{}
	commonPVC, err := getPVCSpec("claim-devworkspace", "test-namespace", nil, resource.MustParse("10Gi"))
	if err != nil {
		t.Fatalf("Failure during setup: %s", err)
	}
	commonPVC.Status.Phase = corev1.ClaimBound
	clusterAPI := sync.ClusterAPI{
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
			err := commonStorage.ProvisionStorage(&tt.Input.PodAdditions, getDevWorkspaceWithConfig(workspace), clusterAPI)
			if tt.Output.ErrRegexp != nil && assert.Error(t, err) {
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error(), "Error message should match")
			} else {
				if !assert.NoError(t, err, "Should not return error") {
					return
				}
				sortVolumesAndVolumeMounts(&tt.Output.PodAdditions)
				sortVolumesAndVolumeMounts(&tt.Input.PodAdditions)
				assert.Equal(t, tt.Output.PodAdditions, tt.Input.PodAdditions,
					"PodAdditions should match expected output: Diff: %s", cmp.Diff(tt.Output.PodAdditions, tt.Input.PodAdditions))
			}
		})
	}
}

func TestTerminatingPVC(t *testing.T) {
	commonStorage := CommonStorageProvisioner{}
	commonPVC, err := getPVCSpec("claim-devworkspace", "test-namespace", nil, resource.MustParse("10Gi"))
	if err != nil {
		t.Fatalf("Failure during setup: %s", err)
	}
	testTime := metav1.Now()
	commonPVC.SetDeletionTimestamp(&testTime)

	clusterAPI := sync.ClusterAPI{
		Client: fake.NewFakeClientWithScheme(scheme, commonPVC),
		Logger: zap.New(),
	}
	testCase := loadTestCaseOrPanic(t, "testdata/common-storage/rewrites-volumes-for-common-pvc-strategy.yaml")
	assert.NotNil(t, testCase.Input.Workspace, "Input does not define workspace")
	workspace := &dw.DevWorkspace{}
	workspace.Spec.Template = *testCase.Input.Workspace
	workspace.Status.DevWorkspaceId = testCase.Input.DevWorkspaceID
	workspace.Namespace = "test-namespace"
	err = commonStorage.ProvisionStorage(&testCase.Input.PodAdditions, getDevWorkspaceWithConfig(workspace), clusterAPI)
	if assert.Error(t, err, "Should return error when PVC is terminating") {
		_, ok := err.(*dwerrors.RetryError)
		assert.True(t, ok, "Expect RetryError when PVC is terminating")
		assert.Equal(t, "Shared PVC is in terminating state", err.Error(), "Expect message that existing PVC is terminating")
	}
}

func TestNeedsStorage(t *testing.T) {
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
								Ephemeral: pointer.Bool(true),
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
								MountSources: pointer.Bool(true),
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
								MountSources: pointer.Bool(true),
							},
						},
					},
				},
				{
					Name: "projects",
					ComponentUnion: dw.ComponentUnion{
						Volume: &dw.VolumeComponent{
							Volume: dw.Volume{
								Ephemeral: pointer.Bool(true),
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
	for idx, container := range podAdditions.InitContainers {
		if container.VolumeMounts != nil {
			sort.Slice(podAdditions.Containers[idx].VolumeMounts, func(i, j int) bool {
				return strings.Compare(podAdditions.Containers[idx].VolumeMounts[i].Name, podAdditions.Containers[idx].VolumeMounts[j].Name) < 0
			})
		}
	}
}
