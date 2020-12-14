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
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devfile/devworkspace-operator/test/e2e/pkg/client"
	"github.com/devfile/devworkspace-operator/test/e2e/pkg/config"
	_ "github.com/devfile/devworkspace-operator/test/e2e/pkg/tests"
	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	"github.com/onsi/gomega"
)

//Create Constant file
const (
	testResultsDirectory = "/tmp/artifacts"
	jUnitOutputFilename  = "junit-workspaces-operator.xml"
	testServiceAccount   = "terminal-test"
)

//SynchronizedBeforeSuite blocks is executed before run all test suites
var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	fmt.Println("Starting to setup objects before run ginkgo suite")

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

	config.OperatorNamespace = "devworkspace-controller"
	config.WorkspaceNamespace = "test-terminal-namespace"

	//create the test workspace for the test user under kube admin

	err = config.AdminK8sClient.CreateNamespace(config.WorkspaceNamespace)
	if err != nil {
		ginkgo.Fail(fmt.Sprintf("Cannot create the namespace: Cause: %s %s", config.WorkspaceNamespace, err.Error()))
	}

	err = config.AdminK8sClient.CreateSA(testServiceAccount, config.WorkspaceNamespace)
	if err != nil {
		ginkgo.Fail("Cannot create test SA. Cause: " + err.Error())

	}
	err = config.AdminK8sClient.AssignRoleToSA(config.WorkspaceNamespace, testServiceAccount, "admin")
	if err != nil {
		ginkgo.Fail("Cannot create test rolebinding for SA. Cause: " + err.Error())
	}

	token, err := config.AdminK8sClient.GetSAToken(config.WorkspaceNamespace, testServiceAccount)
	//sometimes the Service Account token is not applied immediately in this case we wait 1 sec.
	// and try obtain it again
	if err != nil && strings.Contains(err.Error(), "is not found") {
		time.Sleep(1 * time.Second)
		token, err = config.AdminK8sClient.GetSAToken(config.WorkspaceNamespace, testServiceAccount)
	} else if err != nil {
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
		log.Printf("Cleaning up test namespace %s", config.WorkspaceNamespace)
		err := config.AdminK8sClient.DeleteNamespace(config.WorkspaceNamespace)
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Cannot remove the terminal test workspace: %s the error: %s", config.WorkspaceNamespace, err.Error()))
		}
		//TODO 1. Wait until namespace is really removed, otherwise the next e2e run will fail due no objects creation is allowed in Terminating namespace.
		//TODO 2. Alternative - in the set up, before creating a namespace, if it already exists and is terminating - wait until it's removed and recreate it from the scratch
		//TODO To make sure finalizers works as expected the first is better
		_, _ = config.AdminK8sClient.IsNamespaceExist(config.WorkspaceNamespace)
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
