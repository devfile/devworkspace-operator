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

package tests

import (
	"fmt"
	"strings"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/test/e2e/pkg/config"
	"github.com/onsi/ginkgo/v2"
)

var _ = ginkgo.Describe("[DevWorkspace Debug Start Mode]", ginkgo.Ordered, func() {
	defer ginkgo.GinkgoRecover()

	const workspaceName = "code-latest-with-debug-start"

	ginkgo.AfterAll(func() {
		// Clean up workspace and wait for PVC to be fully deleted
		// This prevents PVC conflicts in subsequent tests, especially in CI environments
		_ = config.DevK8sClient.DeleteDevWorkspaceAndWait(workspaceName, config.DevWorkspaceNamespace)
	})

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

	ginkgo.It("Add Debug DevWorkspace to cluster and wait running status", func() {
		commandResult, err := config.DevK8sClient.OcApplyWorkspace(config.DevWorkspaceNamespace, "test/resources/simple-devworkspace-debug-start-annotation.yaml")
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Failed to create DevWorkspace: %s %s", err.Error(), commandResult))
			return
		}

		deploy, err := config.DevK8sClient.WaitDevWsStatus(workspaceName, config.DevWorkspaceNamespace, dw.DevWorkspaceStatusRunning)
		if !deploy {
			ginkgo.Fail(fmt.Sprintf("DevWorkspace didn't start properly. Error: %s", err))
		}
	})

	ginkgo.It("Check DevWorkspace Conditions for Debug Start message", func() {
		devWorkspaceStatus, err := config.DevK8sClient.GetDevWsStatus(workspaceName, config.DevWorkspaceNamespace)
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Failure in fetching DevWorkspace status. Error: %s", err))
		}

		expectedSubstring := "DevWorkspace is starting in debug mode"

		found := false
		for _, cond := range devWorkspaceStatus.Conditions {
			if cond.Message != "" && strings.Contains(cond.Message, expectedSubstring) {
				found = true
				break
			}
		}

		if !found {
			ginkgo.Fail(fmt.Sprintf(
				"DevWorkspace status does not contain expected debug message.\nExpected substring: %q\nGot conditions: %+v",
				expectedSubstring, devWorkspaceStatus.Conditions,
			))
		}
	})
})
