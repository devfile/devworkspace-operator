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

package tests

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/test/e2e/pkg/config"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// getProjectRoot returns the project root directory by navigating up from this file.
func getProjectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "..", "..")
}

var _ = ginkgo.Describe("[Custom Init Container Tests]", func() {
	defer ginkgo.GinkgoRecover()

	ginkgo.It("Wait DevWorkspace Webhook Server Pod", func() {
		controllerLabel := "app.kubernetes.io/name=devworkspace-webhook-server"

		deploy, err := config.AdminK8sClient.WaitForPodRunningByLabel(config.OperatorNamespace, controllerLabel)
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("cannot get the Pod status with label %s: %s", controllerLabel, err.Error()))
			return
		}

		if !deploy {
			ginkgo.Fail("DevWorkspace webhook didn't start properly")
		}
	})

	ginkgo.Context("Custom init-persistent-home container", func() {
		const workspaceName = "custom-init-test"

		ginkgo.BeforeEach(func() {
			dwocFile := filepath.Join(getProjectRoot(), "test", "resources", "dwoc-custom-init.yaml")
			cmd := exec.Command("kubectl", "apply", "-f", dwocFile, "-n", config.OperatorNamespace)
			output, err := cmd.CombinedOutput()
			if err != nil {
				ginkgo.Fail(fmt.Sprintf("Failed to apply DWOC to namespace %s: %s. Output: %s", config.OperatorNamespace, err, string(output)))
			}
		})

		ginkgo.It("Create workspace and verify custom init-persistent-home executed", func() {
			workspaceFile := filepath.Join(getProjectRoot(), "test", "resources", "custom-init-test-workspace.yaml")
			commandResult, err := config.DevK8sClient.OcApplyWorkspace(config.DevWorkspaceNamespace, workspaceFile)
			if err != nil {
				ginkgo.Fail(fmt.Sprintf("Failed to create workspace with custom init: %s %s", err.Error(), commandResult))
				return
			}

			// Wait for workspace to be running
			deploy, err := config.DevK8sClient.WaitDevWsStatus(workspaceName, config.DevWorkspaceNamespace, dw.DevWorkspaceStatusRunning)
			if !deploy {
				ginkgo.Fail(fmt.Sprintf("Workspace didn't start properly. Error: %s", err))
			}

			// Wait for pod to be running
			podSelector := fmt.Sprintf("controller.devfile.io/devworkspace_name=%s", workspaceName)
			var podName string
			gomega.Eventually(func() error {
				podName, err = config.AdminK8sClient.GetPodNameBySelector(podSelector, config.DevWorkspaceNamespace)
				return err
			}, "10m", "5s").Should(gomega.Succeed())

			// Check that the custom init script ran by verifying the marker file exists
			resultOfExecCommand, err := config.DevK8sClient.ExecCommandInContainer(podName, config.DevWorkspaceNamespace, "tooling", "test -f /home/user/.custom_init_complete && echo 'SUCCESS' || echo 'FAILED'")
			if err != nil {
				ginkgo.Fail(fmt.Sprintf("Cannot execute command in container. Error: `%s`. Exec output: `%s`", err, resultOfExecCommand))
			}
			gomega.Expect(resultOfExecCommand).To(gomega.ContainSubstring("SUCCESS"))
		})

		ginkgo.AfterEach(func() {
			// Cleanup workspace
			_ = config.DevK8sClient.DeleteDevWorkspace(workspaceName, config.DevWorkspaceNamespace)
		})
	})

	ginkgo.Context("DisableInitContainer flag behavior", func() {
		const workspaceName = "disabled-init-test"

		ginkgo.BeforeEach(func() {
			dwocFile := filepath.Join(getProjectRoot(), "test", "resources", "dwoc-disabled-init.yaml")
			cmd := exec.Command("kubectl", "apply", "-f", dwocFile, "-n", config.OperatorNamespace)
			output, err := cmd.CombinedOutput()
			if err != nil {
				ginkgo.Fail(fmt.Sprintf("Failed to apply DWOC to namespace %s: %s. Output: %s", config.OperatorNamespace, err, string(output)))
			}
		})

		ginkgo.It("Create workspace and verify no init-persistent-home container present", func() {
			workspaceFile := filepath.Join(getProjectRoot(), "test", "resources", "disabled-init-test-workspace.yaml")
			commandResult, err := config.DevK8sClient.OcApplyWorkspace(config.DevWorkspaceNamespace, workspaceFile)
			if err != nil {
				ginkgo.Fail(fmt.Sprintf("Failed to create workspace: %s %s", err.Error(), commandResult))
				return
			}

			// Wait for workspace to be running
			deploy, err := config.DevK8sClient.WaitDevWsStatus(workspaceName, config.DevWorkspaceNamespace, dw.DevWorkspaceStatusRunning)
			if !deploy {
				ginkgo.Fail(fmt.Sprintf("Workspace didn't start properly. Error: %s", err))
			}

			// Wait for pod to be running (may take a while for large image pulls)
			podSelector := fmt.Sprintf("controller.devfile.io/devworkspace_name=%s", workspaceName)
			var podName string
			gomega.Eventually(func() error {
				podName, err = config.AdminK8sClient.GetPodNameBySelector(podSelector, config.DevWorkspaceNamespace)
				return err
			}, "10m", "5s").Should(gomega.Succeed())

			// Get pod spec and check init containers
			pod, err := config.AdminK8sClient.Kube().CoreV1().Pods(config.DevWorkspaceNamespace).Get(context.TODO(), podName, metav1.GetOptions{})
			if err != nil {
				ginkgo.Fail(fmt.Sprintf("Cannot get pod. Error: %s", err))
			}

			// Verify no init container named init-persistent-home
			for _, initContainer := range pod.Spec.InitContainers {
				if initContainer.Name == "init-persistent-home" {
					ginkgo.Fail("init-persistent-home container should not be present when disableInitContainer is true")
				}
			}
		})

		ginkgo.AfterEach(func() {
			// Cleanup workspace
			_ = config.DevK8sClient.DeleteDevWorkspace(workspaceName, config.DevWorkspaceNamespace)
		})
	})
})
