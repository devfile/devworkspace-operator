// Copyright (c) 2019-2025 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package tests

import (
	"fmt"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/test/e2e/pkg/config"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("[Create DevWorkspace and ensure data is persisted during restarts]", func() {
	defer ginkgo.GinkgoRecover()

	ginkgo.It("Wait DevWorkspace Webhook Server Pod", func() {
		controllerLabel := "app.kubernetes.io/name=devworkspace-webhook-server"

		deploy, err := config.AdminK8sClient.WaitForPodRunningByLabel(config.OperatorNamespace, controllerLabel)
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("cannot get the DevWorkspace Webhook Server Pod status with label %s: %s", controllerLabel, err.Error()))
			return
		}

		if !deploy {
			ginkgo.Fail("Devworkspace webhook didn't start properly")
		}
	})

	ginkgo.It("Add DevWorkspace to cluster and wait running status", func() {
		commandResult, err := config.DevK8sClient.OcApplyWorkspace(config.DevWorkspaceNamespace, "test/resources/simple-devworkspace-with-project-clone.yaml")
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Failed to create DevWorkspace: %s %s", err.Error(), commandResult))
			return
		}

		deploy, err := config.DevK8sClient.WaitDevWsStatus("code-latest", config.DevWorkspaceNamespace, dw.DevWorkspaceStatusRunning)
		if !deploy {
			ginkgo.Fail(fmt.Sprintf("DevWorkspace didn't start properly. Error: %s", err))
		}
	})

	var podName string
	ginkgo.It("Check that project-clone succeeded as expected", func() {
		var err error
		podSelector := "controller.devfile.io/devworkspace_name=code-latest"
		podName, err = config.AdminK8sClient.GetPodNameBySelector(podSelector, config.DevWorkspaceNamespace)
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Can get devworkspace pod by selector. Error: %s", err))
		}
		resultOfExecCommand, err := config.AdminK8sClient.GetLogsForContainer(podName, config.DevWorkspaceNamespace, "project-clone")
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Cannot get logs for project-clone container. Error: `%s`. Exec output: `%s`", err, resultOfExecCommand))
		}
		gomega.Expect(resultOfExecCommand).To(gomega.ContainSubstring("Cloning project web-nodejs-sample to /projects"))
	})

	ginkgo.It("Make some changes in DevWorkspace dev container", func() {
		resultOfExecCommand, err := config.AdminK8sClient.ExecCommandInContainer(podName, config.DevWorkspaceNamespace, "dev", "bash -c 'echo \"## Modified via e2e test\" >> /projects/web-nodejs-sample/README.md'")
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("failed to make changes to DevWorkspace container, returned: %s", resultOfExecCommand))
		}
	})

	ginkgo.It("Stop DevWorkspace", func() {
		err := config.AdminK8sClient.UpdateDevWorkspaceStarted("code-latest", config.DevWorkspaceNamespace, false)
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("failed to stop DevWorkspace container, returned: %s", err))
		}
		deploy, err := config.DevK8sClient.WaitDevWsStatus("code-latest", config.DevWorkspaceNamespace, dw.DevWorkspaceStatusStopped)
		if !deploy {
			ginkgo.Fail(fmt.Sprintf("DevWorkspace didn't start properly. Error: %s", err))
		}
	})

	ginkgo.It("Start DevWorkspace", func() {
		err := config.AdminK8sClient.UpdateDevWorkspaceStarted("code-latest", config.DevWorkspaceNamespace, true)
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("failed to start DevWorkspace container"))
		}
		deploy, err := config.DevK8sClient.WaitDevWsStatus("code-latest", config.DevWorkspaceNamespace, dw.DevWorkspaceStatusRunning)
		if !deploy {
			ginkgo.Fail(fmt.Sprintf("DevWorkspace didn't start properly. Error: %s", err))
		}
	})

	ginkgo.It("Verify changes persist after DevWorkspace restart", func() {
		podSelector := "controller.devfile.io/devworkspace_name=code-latest"
		var err error
		podName, err = config.AdminK8sClient.GetPodNameBySelector(podSelector, config.DevWorkspaceNamespace)
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Can get devworkspace pod by selector. Error: %s", err))
		}
		err = config.AdminK8sClient.WaitForPodContainerToReady(config.DevWorkspaceNamespace, podName, "dev")
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("failed waiting for DevWorkspace container 'dev' to become ready. Error: %s", err))
		}
		resultOfExecCommand, err := config.AdminK8sClient.ExecCommandInContainer(podName, config.DevWorkspaceNamespace, "dev", "bash -c 'cat /projects/web-nodejs-sample/README.md'")
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("failed to verify to DevWorkspace container, returned: %s", resultOfExecCommand))
		}
		gomega.Expect(resultOfExecCommand).To(gomega.ContainSubstring("## Modified via e2e test"))
	})

	ginkgo.It("Check that project-clone logs mention project already cloned", func() {
		resultOfExecCommand, err := config.AdminK8sClient.GetLogsForContainer(podName, config.DevWorkspaceNamespace, "project-clone")
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Cannot get logs for project-clone container. Error: `%s`. Exec output: `%s`", err, resultOfExecCommand))
		}
		gomega.Expect(resultOfExecCommand).To(gomega.ContainSubstring("Project 'web-nodejs-sample' is already cloned and has all remotes configured"))
	})
})
