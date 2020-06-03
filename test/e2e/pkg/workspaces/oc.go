package workspaces

import (
	"fmt"
	"github.com/che-incubator/che-workspace-operator/test/e2e/pkg/metadata"
	"os/exec"
	"strings"
)

func (w *CodeReady) DeployWorkspacesController() (err error) {
	label := "app=che-workspace-controller"
	cmd := exec.Command("oc", "apply", "--namespace", metadata.Namespace.Name, "-f", "deploy/os")
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil && !strings.Contains(string(output), "AlreadyExists") {
		fmt.Println(err)
		return err
	}

	deploy , err := w.PodDeployWaitUtil(label)
	if !deploy {
		fmt.Println("Che Workspaces Controller not deployed")
		return err
	}
	return err
}

func (w *CodeReady) CreateAllOperatorRoles() (err error) {
	cmd := exec.Command("oc", "apply", "--namespace", metadata.Namespace.Name, "-f", "deploy")
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil && !strings.Contains(string(output), "AlreadyExists") {
		fmt.Println(err)
	}
	return err
}

func (w *CodeReady) CreateOpenshiftRoute() (err error) {
	cmd := exec.Command("oc", "apply", "--namespace", metadata.Namespace.Name, "-f", "deploy/registry/local/os")
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil && !strings.Contains(string(output), "AlreadyExists") {
		fmt.Println(err)
	}
	return err
}

func (w *CodeReady) CustomResourceDefinitions() (err error) {
	cmd := exec.Command("oc", "apply", "--namespace", metadata.Namespace.Name, "-f", "deploy/crds")
	output, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(output), "AlreadyExists") {
		fmt.Println(err)
	}
	return err
}

func (w *CodeReady) CreateOperatorClusterRole() (err error) {
	cmd := exec.Command("oc", "apply", "--namespace", metadata.Namespace.Name, "-f", "deploy/role.yaml")
	output, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(output), "AlreadyExists") {
		fmt.Println(err)
	}
	return err
}

func (w *CodeReady) AddCloudShellSample() (err error) {
	const namespace = "che-workspace-controller"
	cmd := exec.Command("oc", "apply", "--namespace", namespace, "-f", "samples/cloud-shell.yaml")
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil && !strings.Contains(string(output), "AlreadyExists") {
		fmt.Println(err)
	}
	return err
}

