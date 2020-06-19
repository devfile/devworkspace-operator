//
// Copyright (c) 2019-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package tests

import (
	"fmt"

	"github.com/devfile/devworkspace-operator/test/e2e/pkg/client"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("[Create Cloud Shell Workspace]", func() {
	ginkgo.It("Add cloud shell to cluster", func() {
		label := "controller.devfile.io/workspace_name=cloud-shell"
		k8sClient, err := client.NewK8sClient()
		if err != nil {
			ginkgo.Fail("Failed to create k8s client: " + err.Error())
			return
		}
		err = k8sClient.OcApplyWorkspace("samples/cloud-shell.yaml")
		if err != nil {
			ginkgo.Fail("Failed to create cloud-shell devworkspace: " + err.Error())
			return
		}
		deploy, err := k8sClient.WaitForPodRunningByLabel(label)

		if !deploy {
			fmt.Println("Cloud Shell not deployed")
		}
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	})
})
