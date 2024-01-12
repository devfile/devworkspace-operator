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

package cmd

import (
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/devfile/devworkspace-operator/test/e2e/pkg/client"
	"github.com/devfile/devworkspace-operator/test/e2e/pkg/config"
	_ "github.com/devfile/devworkspace-operator/test/e2e/pkg/tests"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// Create Constant file
const (
	testResultsDirectory = "/tmp/artifacts"
	jUnitOutputFilename  = "junit-workspaces-operator.xml"
	testServiceAccount   = "terminal-test"
)

// SynchronizedBeforeSuite blocks is executed before run all test suites
var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	log.Println("Starting to setup objects before run ginkgo suite")

	var err error
	kubeConfig := os.Getenv("KUBECONFIG")

	if len(kubeConfig) == 0 {
		failMess := "The mandatory environment variable(s) is not set.\nMake sure that all variables have been set properly. " +
			"The variable list:\nKUBECONFIG=%s"
		ginkgo.Fail(fmt.Sprintf(failMess, kubeConfig))
	}

	config.AdminK8sClient, err = client.NewK8sClientWithKubeConfig(kubeConfig)

	if err != nil {
		ginkgo.Fail("Cannot create admin k8s client. Cause: " + err.Error())
	}

	operatorNamespace := os.Getenv("NAMESPACE")
	if operatorNamespace != "" {
		config.OperatorNamespace = operatorNamespace
	} else {
		config.OperatorNamespace = "devworkspace-controller"
	}
	config.DevWorkspaceNamespace = "test-terminal-namespace"

	//create the test workspace for the test user under kube admin

	err = config.AdminK8sClient.CreateNamespace(config.DevWorkspaceNamespace)
	if err != nil {
		ginkgo.Fail(fmt.Sprintf("Cannot create the namespace %q: Cause: %s", config.DevWorkspaceNamespace, err.Error()))
	}

	err = config.AdminK8sClient.CreateSA(testServiceAccount, config.DevWorkspaceNamespace)
	if err != nil {
		ginkgo.Fail("Cannot create test SA. Cause: " + err.Error())

	}
	err = config.AdminK8sClient.AssignRoleToSA(config.DevWorkspaceNamespace, testServiceAccount, "admin")
	if err != nil {
		ginkgo.Fail("Cannot create test rolebinding for SA. Cause: " + err.Error())
	}

	token, err := config.AdminK8sClient.WaitSAToken(config.DevWorkspaceNamespace, testServiceAccount)
	if err != nil {
		ginkgo.Fail("Cannot get test SA token. Cause: " + err.Error())
	}

	config.DevK8sClient, err = client.NewK8sClientWithToken(kubeConfig, token)
	if err != nil {
		ginkgo.Fail("Cannot create k8s client for the test ServiceAccount " + err.Error())
	}

	return nil
}, func(data []byte) {})

var _ = ginkgo.SynchronizedAfterSuite(func() {
	cleanUpAfterSuite := os.Getenv("CLEAN_UP_AFTER_SUITE")
	//clean up by default or when user configured it explicitly
	if cleanUpAfterSuite == "" || cleanUpAfterSuite == "true" {
		log.Printf("Cleaning up test namespace %s", config.DevWorkspaceNamespace)
		log.Printf("If you need resources for investigation, set the following env var CLEAN_UP_AFTER_SUITE=false")
		err := config.AdminK8sClient.DeleteNamespace(config.DevWorkspaceNamespace)
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Failed to remove test namespace '%s'. Cause: %s", config.DevWorkspaceNamespace, err.Error()))
		}
		err = config.AdminK8sClient.WaitNamespaceIsTerminated(config.DevWorkspaceNamespace)
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Test namespace '%s' is not cleaned up after test. Cause: %s", config.DevWorkspaceNamespace, err.Error()))
		}
	} else {
		log.Printf("Cleaning up test resources are disabled")
	}
}, func() {})

func TestWorkspaceController(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)

	log.Println("Running DevWorkspace Controller e2e tests...")
	ginkgo.RunSpecs(t, "Workspaces Controller Operator Tests")
}
