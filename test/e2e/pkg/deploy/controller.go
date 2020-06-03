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

package deploy

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/che-incubator/che-workspace-operator/test/e2e/pkg/config"
)

func (w *Deployment) DeployWorkspacesController() (err error) {
	label := "app=che-workspace-controller"
	cmd := exec.Command("oc", "apply", "--namespace", config.Namespace, "-f", "deploy/os")
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil && !strings.Contains(string(output), "AlreadyExists") {
		fmt.Println(err)
		return err
	}

	deploy, err := w.kubeClient.PodDeployWaitUtil(label)
	if !deploy {
		fmt.Println("Che Workspaces Controller not deployed")
		return err
	}
	return err
}

func (w *Deployment) CreateAllOperatorRoles() (err error) {
	cmd := exec.Command("oc", "apply", "--namespace", config.Namespace, "-f", "deploy")
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil && !strings.Contains(string(output), "AlreadyExists") {
		fmt.Println(err)
	}
	return err
}

func (w *Deployment) CustomResourceDefinitions() (err error) {
	cmd := exec.Command("oc", "apply", "-f", "deploy/crds")
	output, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(output), "AlreadyExists") {
		fmt.Println(err)
	}
	return err
}

func (w *Deployment) CreateOperatorClusterRole() (err error) {
	cmd := exec.Command("oc", "apply", "--namespace", config.Namespace, "-f", "deploy/role.yaml")
	output, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(output), "AlreadyExists") {
		fmt.Println(err)
	}
	return err
}
