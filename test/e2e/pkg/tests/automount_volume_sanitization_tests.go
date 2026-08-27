//
// Copyright (c) 2019-2026 Red Hat, Inc.
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

package tests

import (
	"context"
	"fmt"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/test/e2e/pkg/config"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Verifies that a secret whose name contains characters invalid in a volume name
// (e.g. dots) is auto-mounted with a sanitized, DNS-1123 compliant volume name.
var _ = ginkgo.Describe("[Automount Secret with Invalid Volume Name Characters]", ginkgo.Ordered, func() {
	defer ginkgo.GinkgoRecover()

	const (
		workspaceName      = "volume-sanitization-test"
		secretWithDots     = "test.pullsecret" // Invalid: contains dots
		expectedVolumeName = "test-pullsecret" // Expected sanitized name
		secretData         = "test-secret-data"
	)

	ginkgo.AfterAll(func() {
		// Delete the test secret
		_ = config.DevK8sClient.Kube().CoreV1().Secrets(config.DevWorkspaceNamespace).
			Delete(context.TODO(), secretWithDots, metav1.DeleteOptions{})

		// Cleanup workspace and wait for PVC to be fully deleted
		// This prevents PVC conflicts in subsequent tests, especially in CI environments
		_ = config.DevK8sClient.DeleteDevWorkspaceAndWait(workspaceName, config.DevWorkspaceNamespace)
	})

	ginkgo.It("Create secret with dots in name and mount-to-devworkspace label", func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretWithDots,
				Namespace: config.DevWorkspaceNamespace,
				Labels: map[string]string{
					"controller.devfile.io/mount-to-devworkspace": "true",
					"controller.devfile.io/watch-secret":          "true",
				},
			},
			StringData: map[string]string{
				"test-key": secretData,
			},
			Type: corev1.SecretTypeOpaque,
		}

		_, err := config.DevK8sClient.Kube().CoreV1().Secrets(config.DevWorkspaceNamespace).
			Create(context.TODO(), secret, metav1.CreateOptions{})
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Failed to create secret with dots in name: %s", err.Error()))
		}
	})

	ginkgo.It("Create and start DevWorkspace that should auto-mount the secret", func() {
		commandResult, err := config.DevK8sClient.OcApplyWorkspace(
			config.DevWorkspaceNamespace,
			"test/resources/volume-sanitization-test-workspace.yaml",
		)
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Failed to create DevWorkspace: %s %s", err.Error(), commandResult))
		}
	})

	ginkgo.It("Wait for DevWorkspace to reach Running status", func() {
		deploy, err := config.DevK8sClient.WaitDevWsStatus(
			workspaceName,
			config.DevWorkspaceNamespace,
			dw.DevWorkspaceStatusRunning,
		)
		if !deploy {
			ginkgo.Fail(fmt.Sprintf("DevWorkspace didn't start properly. Error: %s", err))
		}
	})

	var podName string
	ginkgo.It("Verify deployment has sanitized volume name", func() {
		podSelector := fmt.Sprintf("controller.devfile.io/devworkspace_name=%s", workspaceName)
		var err error
		podName, err = config.AdminK8sClient.GetPodNameBySelector(podSelector, config.DevWorkspaceNamespace)
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Cannot get workspace pod by selector. Error: %s", err))
		}

		pod, err := config.DevK8sClient.Kube().CoreV1().Pods(config.DevWorkspaceNamespace).
			Get(context.TODO(), podName, metav1.GetOptions{})
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Failed to get pod: %s", err.Error()))
		}

		// Verify volume name is sanitized (dots replaced with hyphens)
		volumeFound := false
		for _, volume := range pod.Spec.Volumes {
			if volume.Name == expectedVolumeName {
				volumeFound = true
				// Verify it's a secret volume with the correct secret name
				if volume.Secret == nil {
					ginkgo.Fail(fmt.Sprintf("Volume %s is not a secret volume", expectedVolumeName))
				}
				if volume.Secret.SecretName != secretWithDots {
					ginkgo.Fail(fmt.Sprintf("Volume %s references wrong secret: %s, expected: %s",
						expectedVolumeName, volume.Secret.SecretName, secretWithDots))
				}
				break
			}
			// Also verify the original name (with dots) is NOT used
			if volume.Name == secretWithDots {
				ginkgo.Fail(fmt.Sprintf("Volume name was not sanitized: found volume with name '%s' (should be '%s')",
					secretWithDots, expectedVolumeName))
			}
		}

		if !volumeFound {
			ginkgo.Fail(fmt.Sprintf("Sanitized volume name '%s' not found in pod volumes. Available volumes: %v",
				expectedVolumeName, getPodVolumeNames(pod)))
		}
	})

	ginkgo.It("Verify volume mount uses sanitized volume name", func() {
		pod, err := config.DevK8sClient.Kube().CoreV1().Pods(config.DevWorkspaceNamespace).
			Get(context.TODO(), podName, metav1.GetOptions{})
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Failed to get pod: %s", err.Error()))
		}

		// Check all containers for the volume mount
		volumeMountFound := false
		for _, container := range pod.Spec.Containers {
			for _, volumeMount := range container.VolumeMounts {
				if volumeMount.Name == expectedVolumeName {
					volumeMountFound = true
					// Verify mount path
					expectedMountPath := fmt.Sprintf("/etc/secret/%s", secretWithDots)
					if volumeMount.MountPath != expectedMountPath {
						ginkgo.Fail(fmt.Sprintf("Volume mount path incorrect: got %s, expected %s",
							volumeMount.MountPath, expectedMountPath))
					}
					break
				}
			}
		}

		if !volumeMountFound {
			ginkgo.Fail(fmt.Sprintf("Volume mount with sanitized name '%s' not found in any container",
				expectedVolumeName))
		}
	})

	ginkgo.It("Verify secret data is accessible inside the container", func() {
		// Execute command to verify the secret is mounted and accessible
		containerName := "test-container"
		secretFilePath := fmt.Sprintf("/etc/secret/%s/test-key", secretWithDots)
		catCommand := fmt.Sprintf("cat %s", secretFilePath)

		resultOfExecCommand, err := config.DevK8sClient.ExecCommandInContainer(
			podName,
			config.DevWorkspaceNamespace,
			containerName,
			catCommand,
		)
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Cannot execute command in the devworkspace container. Error: `%s`. Exec output: `%s`",
				err, resultOfExecCommand))
		}

		gomega.Expect(resultOfExecCommand).To(gomega.ContainSubstring(secretData))
	})
})

// Helper function to get pod volume names for debugging
func getPodVolumeNames(pod *corev1.Pod) []string {
	volumeNames := make([]string, len(pod.Spec.Volumes))
	for i, volume := range pod.Spec.Volumes {
		volumeNames[i] = volume.Name
	}
	return volumeNames
}
