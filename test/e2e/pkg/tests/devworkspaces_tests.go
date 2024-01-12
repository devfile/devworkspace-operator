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

package tests

import (
	"fmt"
	"strings"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/test/e2e/pkg/config"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("[Create OpenShift Web Terminal Workspace]", func() {
	defer ginkgo.GinkgoRecover()

	ginkgo.It("Wait DewWorkspace Webhook Server Pod", func() {
		controllerLabel := "app.kubernetes.io/name=devworkspace-webhook-server"

		deploy, err := config.AdminK8sClient.WaitForPodRunningByLabel(config.OperatorNamespace, controllerLabel)
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("cannot get the Pod status with label %s: %s", controllerLabel, err.Error()))
			return
		}

		if !deploy {
			ginkgo.Fail("Devworkspace webhook  didn't start properly")
		}
	})

	ginkgo.It("Add OpenShift web terminal to cluster and wait running status", func() {
		commandResult, err := config.DevK8sClient.OcApplyWorkspace(config.DevWorkspaceNamespace, "test/resources/restricted-access-devworkspace.yaml")
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Failed to create OpenShift web terminal workspace: %s %s", err.Error(), commandResult))
			return
		}

		deploy, err := config.DevK8sClient.WaitDevWsStatus("restricted-access", config.DevWorkspaceNamespace, dw.DevWorkspaceStatusRunning)
		if !deploy {
			ginkgo.Fail(fmt.Sprintf("OpenShift Web terminal workspace didn't start properly. Error: %s", err))
		}
	})

	var podName string
	ginkgo.It("Check that pod creator can execute a command in the container", func() {
		podSelector := "controller.devfile.io/devworkspace_name=restricted-access"
		var err error
		podName, err = config.AdminK8sClient.GetPodNameBySelector(podSelector, config.DevWorkspaceNamespace)
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Can get web terminal pod by selector. Error: %s", err))
		}
		resultOfExecCommand, err := config.DevK8sClient.ExecCommandInContainer(podName, config.DevWorkspaceNamespace, "echo hello dev")
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Cannot execute command in the devworkspace container. Error: `%s`. Exec output: `%s`", err, resultOfExecCommand))
		}
		gomega.Expect(resultOfExecCommand).To(gomega.ContainSubstring("hello dev"))
	})

	ginkgo.It("Check that not pod owner cannot execute a command in the container", func() {
		resultOfExecCommand, err := config.AdminK8sClient.ExecCommandInContainer(podName, config.DevWorkspaceNamespace, "echo hello dev")
		if err == nil {
			ginkgo.Fail(fmt.Sprintf("Admin is not supposed to be able to exec into test terminal but exec is executed successfully and returned: %s", resultOfExecCommand))
		}
		if !strings.Contains(resultOfExecCommand, "denied the request: The only devworkspace creator has exec access") {
			ginkgo.Fail(fmt.Sprintf("Exec command is failed due different reason than expected restricted access. Error: `%s`. Exec output: `%s`", err, resultOfExecCommand))
		}
		// as expected exec is failed due restricted access
	})
})
