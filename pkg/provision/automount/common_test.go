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

package automount

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
)

type mountedVolumeType int

const (
	devWorkspaceVolume mountedVolumeType = iota
	secretVolumeType
	configMapVolumeType
)

const (
	testContainerName = "testContainer"
	testNamespace     = "test-namespace"
)

type testCase struct {
	Name  string `json:"name"`
	Input struct {
		// Secrets and Configmaps are necessary for deserialization from a testcase
		Secrets    []corev1.Secret    `json:"secrets"`
		ConfigMaps []corev1.ConfigMap `json:"configmaps"`
		// allObjects contains all Secrets and Configmaps defined above, for convenience
		allObjects []client.Object
	} `json:"input"`
	Output struct {
		// List of volumes expected in the resulting podAdditions; if the name of any volume
		// starts with '/', the name will be overwritten to common.AutoMountProjectedVolumeName(name)
		Volumes []corev1.Volume `json:"volumes"`
		// List of volumeMounts expected in the resulting podAdditions; if the name of any volumeMount
		// starts with '/', the name will be overwritten to common.AutoMountProjectedVolumeName(name)
		VolumeMounts []corev1.VolumeMount   `json:"volumeMounts"`
		EnvFrom      []corev1.EnvFromSource `json:"envFrom"`
		ErrRegexp    *string                `json:"errRegexp"`
	} `json:"output"`
	TestPath string
}

var testDiffOpts = cmp.Options{
	cmpopts.SortSlices(func(a, b corev1.Volume) bool {
		return a.Name < b.Name
	}),
	cmpopts.SortSlices(func(a, b corev1.VolumeMount) bool {
		if a.Name == b.Name {
			return a.MountPath < b.MountPath
		}
		return a.Name < b.Name
	}),
	cmpopts.SortSlices(func(a, b corev1.EnvFromSource) bool {
		switch {
		case a.ConfigMapRef != nil && b.ConfigMapRef != nil:
			return a.ConfigMapRef.Name < b.ConfigMapRef.Name
		case a.ConfigMapRef != nil && b.ConfigMapRef == nil:
			return true
		case a.ConfigMapRef == nil && b.ConfigMapRef != nil:
			return false
		default:
			return a.SecretRef.Name < b.SecretRef.Name
		}
	}),
	cmpopts.SortSlices(func(a, b corev1.VolumeProjection) bool {
		switch {
		case a.ConfigMap != nil && b.ConfigMap != nil:
			return a.ConfigMap.Name < b.ConfigMap.Name
		case a.ConfigMap == nil && b.ConfigMap != nil:
			return true
		case a.ConfigMap != nil && b.ConfigMap == nil:
			return false
		default:
			return a.Secret.Name < b.Secret.Name
		}
	}),
}

func TestProvisionAutomountResourcesInto(t *testing.T) {
	tests := loadAllTestCasesOrPanic(t, "testdata")
	testContainer := corev1.Container{
		Name:  "test-container",
		Image: "test-image",
	}
	testPodAdditions := &v1alpha1.PodAdditions{
		Containers:     []corev1.Container{testContainer},
		InitContainers: []corev1.Container{testContainer},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s (%s)", tt.Name, tt.TestPath), func(t *testing.T) {
			podAdditions := testPodAdditions.DeepCopy()
			testAPI := sync.ClusterAPI{
				Client: fake.NewClientBuilder().WithObjects(tt.Input.allObjects...).Build(),
			}
			// Note: this test does not allow for returning AutoMountError with isFatal: false (i.e. no retrying)
			// and so is not suitable for testing automount features that provision cluster resources (yet)
			err := ProvisionAutoMountResourcesInto(podAdditions, testAPI, testNamespace)
			if tt.Output.ErrRegexp != nil {
				if !assert.Error(t, err, "Expected an error but got none") {
					return
				}
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error(), "Expected error messages to match")
			} else {
				if !assert.NoError(t, err, "Unexpected error") {
					return
				}
				assert.Truef(t, cmp.Equal(tt.Output.Volumes, podAdditions.Volumes, testDiffOpts),
					"Volumes should match expected output:\n%s",
					cmp.Diff(tt.Output.Volumes, podAdditions.Volumes, testDiffOpts))

				for _, container := range podAdditions.Containers {
					assert.Truef(t, cmp.Equal(tt.Output.VolumeMounts, container.VolumeMounts, testDiffOpts),
						"Container VolumeMounts should match expected output:\n%s",
						cmp.Diff(tt.Output.VolumeMounts, container.VolumeMounts, testDiffOpts))
					assert.Truef(t, cmp.Equal(tt.Output.EnvFrom, container.EnvFrom, testDiffOpts),
						"Container EnvFrom should match expected output:\n%s",
						cmp.Diff(tt.Output.EnvFrom, container.EnvFrom, testDiffOpts))
				}

				for _, container := range podAdditions.InitContainers {
					assert.Truef(t, cmp.Equal(tt.Output.VolumeMounts, container.VolumeMounts, testDiffOpts),
						"Container VolumeMounts should match expected output:\n%s",
						cmp.Diff(tt.Output.VolumeMounts, container.VolumeMounts, testDiffOpts))
					assert.Truef(t, cmp.Equal(tt.Output.EnvFrom, container.EnvFrom, testDiffOpts),
						"Container EnvFrom should match expected output:\n%s",
						cmp.Diff(tt.Output.EnvFrom, container.EnvFrom, testDiffOpts))
				}
			}
		})
	}
}

func TestCheckAutoMountVolumesForCollision(t *testing.T) {
	type volumeDesc struct {
		name       string
		mountPath  string
		volumeType mountedVolumeType
	}
	tests := []struct {
		name                  string
		basePodAdditions      []volumeDesc
		automountPodAdditions []volumeDesc
		errRegexp             string
	}{
		{
			name: "Does not error when mounts are valid",
			basePodAdditions: []volumeDesc{
				{
					name:       "baseVolume",
					mountPath:  "basePath",
					volumeType: configMapVolumeType,
				},
			},
			automountPodAdditions: []volumeDesc{
				{
					name:       "automountConfigMap",
					mountPath:  "/configmap/mount",
					volumeType: configMapVolumeType,
				},
				{
					name:       "automountSecret",
					mountPath:  "/secret/mount",
					volumeType: secretVolumeType,
				},
			},
		},
		{
			name: "Detects volume name collision",
			basePodAdditions: []volumeDesc{
				{
					name:       "baseVolume",
					mountPath:  "basePath",
					volumeType: devWorkspaceVolume,
				},
			},
			automountPodAdditions: []volumeDesc{
				{
					name:       "baseVolume",
					mountPath:  "/configmap/mount",
					volumeType: configMapVolumeType,
				},
			},
			errRegexp: "DevWorkspace volume 'baseVolume' conflicts with automounted volume from configmap 'baseVolume'",
		},
		{
			name: "Detects mountPath collision with DevWorkspace",
			basePodAdditions: []volumeDesc{
				{
					name:       "baseVolume",
					mountPath:  "/collision/path",
					volumeType: devWorkspaceVolume,
				},
			},
			automountPodAdditions: []volumeDesc{
				{
					name:       "testVolume",
					mountPath:  "/collision/path",
					volumeType: secretVolumeType,
				},
			},
			errRegexp: fmt.Sprintf("DevWorkspace volume 'baseVolume' in container %s has same mountpath as auto-mounted volume from secret 'testVolume'", testContainerName),
		},
		{
			name: "Detects mountPath collision in automounted volumes",
			automountPodAdditions: []volumeDesc{
				{
					name:       "testVolume1",
					mountPath:  "/test/mount",
					volumeType: secretVolumeType,
				},
				{
					name:       "testVolume2",
					mountPath:  "/test/mount",
					volumeType: configMapVolumeType,
				},
			},
			errRegexp: "auto-mounted volumes from configmap 'testVolume2' and secret 'testVolume1' have the same mount path",
		},
	}

	convertDescToVolume := func(desc volumeDesc) (*corev1.Volume, *corev1.VolumeMount, *corev1.Container) {
		switch desc.volumeType {
		case secretVolumeType:
			volume := &corev1.Volume{
				Name: desc.name,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: desc.name,
					},
				},
			}
			volumeMount := &corev1.VolumeMount{
				Name:      desc.name,
				MountPath: desc.mountPath,
			}
			return volume, volumeMount, nil
		case configMapVolumeType:
			volume := &corev1.Volume{
				Name: desc.name,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: desc.name,
						},
					},
				},
			}
			volumeMount := &corev1.VolumeMount{
				Name:      desc.name,
				MountPath: desc.mountPath,
			}
			return volume, volumeMount, nil
		case devWorkspaceVolume:
			volume := &corev1.Volume{
				Name: desc.name,
			}
			container := &corev1.Container{
				Name: testContainerName,
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      desc.name,
						MountPath: desc.mountPath,
					},
				},
			}
			return volume, nil, container
		}
		return nil, nil, nil
	}

	convertToPodAddition := func(descs ...volumeDesc) *v1alpha1.PodAdditions {
		pa := &v1alpha1.PodAdditions{}
		for _, desc := range descs {
			volume, volumeMount, container := convertDescToVolume(desc)
			if volume != nil {
				pa.Volumes = append(pa.Volumes, *volume)
			}
			if volumeMount != nil {
				pa.VolumeMounts = append(pa.VolumeMounts, *volumeMount)
			}
			if container != nil {
				pa.Containers = append(pa.Containers, *container)
			}
		}
		return pa
	}

	convertToAutomountResources := func(descs ...volumeDesc) *Resources {
		resources := &Resources{}
		for _, desc := range descs {
			volume, volumeMount, _ := convertDescToVolume(desc)
			if volume != nil {
				resources.Volumes = append(resources.Volumes, *volume)
			}
			if volumeMount != nil {
				resources.VolumeMounts = append(resources.VolumeMounts, *volumeMount)
			}
		}
		return resources
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			base := convertToPodAddition(tt.basePodAdditions...)

			autoMount := convertToAutomountResources(tt.automountPodAdditions...)

			outErr := checkAutomountVolumesForCollision(base, autoMount)
			if tt.errRegexp == "" {
				assert.Nil(t, outErr, "Expected no error but got %s", outErr)
			} else {
				assert.NotNil(t, outErr, "Expected error but got nil")
				assert.Regexp(t, tt.errRegexp, outErr, "Error message should match regexp %s", tt.errRegexp)
			}
		})
	}
}

func loadAllTestCasesOrPanic(t *testing.T, fromDir string) []testCase {
	files, err := os.ReadDir(fromDir)
	if err != nil {
		t.Fatal(err)
	}
	var tests []testCase
	for _, file := range files {
		if file.IsDir() {
			tests = append(tests, loadAllTestCasesOrPanic(t, filepath.Join(fromDir, file.Name()))...)
		} else {
			tests = append(tests, loadTestCaseOrPanic(t, filepath.Join(fromDir, file.Name())))
		}
	}
	return tests
}

func loadTestCaseOrPanic(t *testing.T, testPath string) testCase {
	bytes, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatal(err)
	}
	var test testCase
	if err := yaml.Unmarshal(bytes, &test); err != nil {
		t.Fatal(err)
	}

	// Go doesn't allow conversion to interfaces (e.g. client.Object) for elements of slices,
	// so we have to add one at a time
	for idx := range test.Input.ConfigMaps {
		test.Input.allObjects = append(test.Input.allObjects, &test.Input.ConfigMaps[idx])
	}
	for idx := range test.Input.Secrets {
		test.Input.allObjects = append(test.Input.allObjects, &test.Input.Secrets[idx])
	}

	// Overwrite namespace for convenience
	for _, obj := range test.Input.allObjects {
		obj.SetNamespace(testNamespace)
	}

	// Overwrite volume and volumeMount names for projected volumes
	for idx, vol := range test.Output.Volumes {
		if strings.HasPrefix(vol.Name, "/") {
			test.Output.Volumes[idx].Name = common.AutoMountProjectedVolumeName(vol.Name)
		}
	}
	for idx, vm := range test.Output.VolumeMounts {
		if strings.HasPrefix(vm.Name, "/") {
			test.Output.VolumeMounts[idx].Name = common.AutoMountProjectedVolumeName(vm.Name)
		}
	}

	test.TestPath = testPath
	return test
}
