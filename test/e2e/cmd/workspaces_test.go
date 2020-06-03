package cmd

import (
	"fmt"
	"path/filepath"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/che-incubator/che-workspace-operator/test/e2e/pkg/metadata"
	_ "github.com/che-incubator/che-workspace-operator/test/e2e/pkg/tests"
	workspaces "github.com/che-incubator/che-workspace-operator/test/e2e/pkg/workspaces"
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
	//!TODO: Try to create a specific function to call all <ginkgo suite> configuration.
	fmt.Println("Starting to setup objects before run ginkgo suite")
	metadata.Namespace.Name = "che-workspace-controller"

	workspaces := workspaces.NewWorkspaceClient()

	ns := newNamespace()
	ns, err := workspaces.Kube().CoreV1().Namespaces().Create(ns)

	if err != nil {
		fmt.Println("Failed to create namespace")
	}

	if err := workspaces.CreatePluginRegistryDeployment(); err != nil {
		_ = fmt.Errorf("Failed to create deployment for plugin registry")
	}

	if err := workspaces.CreatePluginRegistryService(); err != nil {
		_ = fmt.Errorf("Failed to create plugin registry service %s", err)
	}

	if err := workspaces.CreateOpenshiftRoute(); err != nil {
		_ = fmt.Errorf("Failed to create route in cluster %s", err)
	}

	if err := workspaces.CreateAllOperatorRoles(); err != nil {
		_ = fmt.Errorf("Failed to create roles in clusters %s", err)
	}

	if err := workspaces.CreateOperatorClusterRole(); err != nil {
		_ = fmt.Errorf("Failed to create roles in clusters %s", err)
	}

	if err := workspaces.CustomResourceDefinitions(); err != nil {
		_ = fmt.Errorf("Failed to add custom resources definitions to cluster %s", err)
	}

	if err := workspaces.DeployWorkspacesController(); err != nil {
		_ = fmt.Errorf("Failed to deploy workspace controller %s", err)
	}

	return nil
}, func(data []byte) {})

var _ = ginkgo.SynchronizedAfterSuite(func() {
	workspaces := workspaces.NewWorkspaceClient()

	if err := workspaces.Kube().CoreV1().Namespaces().Delete(metadata.Namespace.Name, &metav1.DeleteOptions{}); err != nil {
		_ = fmt.Errorf("Failed to deploy workspace controller %s", err)
	}
}, func() {})

func TestHarnessCodeReadyWorkspaces(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)

	fmt.Println("Creating ginkgo reporter for Test Harness: Junit and Debug Detail reporter")
	var r []ginkgo.Reporter
	r = append(r, reporters.NewJUnitReporter(filepath.Join(testResultsDirectory, jUnitOutputFilename)))

	fmt.Println("Running Workspace Controller e2e tests...")
	ginkgo.RunSpecsWithDefaultAndCustomReporters(t, "Workspaces Controller Operator Tests", r)

}

func newNamespace() (ns *corev1.Namespace) {
	return &corev1.Namespace{

		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: corev1.SchemeGroupVersion.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: metadata.Namespace.Name,
		},
	}
}
