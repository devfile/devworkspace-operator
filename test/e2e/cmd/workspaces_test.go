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

package cmd

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/devfile/devworkspace-operator/test/e2e/pkg/config"
	"github.com/devfile/devworkspace-operator/test/e2e/pkg/deploy"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/devfile/devworkspace-operator/test/e2e/pkg/client"
	_ "github.com/devfile/devworkspace-operator/test/e2e/pkg/tests"
	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	"github.com/onsi/gomega"
)

//Create Constant file
const (
	testResultsDirectory = "/tmp/artifacts"
	jUnitOutputFilename  = "junit-workspaces-operator.xml"
)

//SynchronizedBeforeSuite blocks is executed before run all test suites
var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	fmt.Println("Starting to setup objects before run ginkgo suite")
	config.Namespace = "che-workspace-controller"

	k8sClient, err := client.NewK8sClient()
	if err != nil {
		fmt.Println("Failed to create workspace client")
		panic(err)
	}

	controller := deploy.NewDeployment(k8sClient)

	err = controller.CreateNamespace()
	if err != nil {
		panic(err)
	}

	if err := controller.CreateAdditionalControllerResources(); err != nil {
		fmt.Println("Failed to create additional controller resources in clusters")
		panic(err)
	}

	if err := controller.CustomResourceDefinitions(); err != nil {
		fmt.Println("Failed to add custom resources definitions to cluster")
		panic(err)
	}

	if err := controller.DeployWorkspacesController(); err != nil {
		fmt.Println("Failed to deploy workspace controller")
		panic(err)
	}

	return nil
}, func(data []byte) {})

var _ = ginkgo.SynchronizedAfterSuite(func() {
	k8sClient, err := client.NewK8sClient()

	if err != nil {
		_ = fmt.Errorf("Failed to uninstall workspace controller %s", err)
	}

	if err = k8sClient.Kube().CoreV1().Namespaces().Delete(config.Namespace, &metav1.DeleteOptions{}); err != nil {
		_ = fmt.Errorf("Failed to uninstall workspace controller %s", err)
	}
}, func() {})

func TestWorkspaceController(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)

	fmt.Println("Creating ginkgo reporter for Test Harness: Junit and Debug Detail reporter")
	var r []ginkgo.Reporter
	r = append(r, reporters.NewJUnitReporter(filepath.Join(testResultsDirectory, jUnitOutputFilename)))

	fmt.Println("Running Workspace Controller e2e tests...")
	ginkgo.RunSpecsWithDefaultAndCustomReporters(t, "Workspaces Controller Operator Tests", r)
}
