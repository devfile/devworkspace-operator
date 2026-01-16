//
// Copyright (c) 2019-2025 Red Hat, Inc.
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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
)

func TestProvisionAutomountResourcesIntoPersistentHomeEnabled(t *testing.T) {
	tests := []testCase{
		loadTestCaseOrPanic(t, "testdata/testProvisionsConfigmaps.yaml"),
		loadTestCaseOrPanic(t, "testdata/testProvisionsProjectedVolumes.yaml"),
		loadTestCaseOrPanic(t, "testdata/testProvisionsSecrets.yaml"),
	}

	testPodAdditions := &v1alpha1.PodAdditions{
		Containers: []corev1.Container{{
			Name:  "test-container",
			Image: "test-image",
		}},
		InitContainers: []corev1.Container{{
			Name:  "init-persistent-home",
			Image: "test-image",
		}, {
			Name:  "test-container",
			Image: "test-image",
		}},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s (%s)", tt.Name, tt.TestPath), func(t *testing.T) {
			podAdditions := testPodAdditions.DeepCopy()
			testAPI := sync.ClusterAPI{
				Client: fake.NewClientBuilder().WithObjects(tt.Input.allObjects...).Build(),
			}

			err := ProvisionAutoMountResourcesInto(podAdditions, testAPI, testNamespace, true, false)

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
				if container.Name == "init-persistent-home" {
					assert.Truef(t, container.VolumeMounts == nil || len(container.VolumeMounts) == 0,
						"The init-persistent-home container should not have any volume mounts if persistent home is enabled")
				} else {
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
