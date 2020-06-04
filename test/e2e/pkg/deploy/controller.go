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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os/exec"
	"strings"

	"github.com/che-incubator/che-workspace-operator/test/e2e/pkg/config"
)

func (w *Deployment) CreateNamespace() error {
	_, err := w.kubeClient.Kube().CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.Namespace,
		},
	})
	return err
}

func (w *Deployment) DeployWorkspacesController() error {
	label := "app=che-workspace-controller"
	cmd := exec.Command("oc", "apply", "--namespace", config.Namespace, "-f", "deploy/os")
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil && !strings.Contains(string(output), "AlreadyExists") {
		fmt.Println(err)
		return err
	}

	deploy, err := w.kubeClient.WaitForPodRunningByLabel(label)
	if !deploy || err != nil {
		fmt.Println("Che Workspaces Controller not deployed")
		return err
	}
	return nil
}

func (w *Deployment) CreateAllOperatorRoles() error {
	cmd := exec.Command("oc", "apply", "--namespace", config.Namespace, "-f", "deploy")
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil && !strings.Contains(string(output), "AlreadyExists") {
		fmt.Println(err)
		return err
	}
	return nil
}

func (w *Deployment) CustomResourceDefinitions() error {
	cmd := exec.Command("oc", "apply", "-f", "deploy/crds")
	output, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(output), "AlreadyExists") {
		fmt.Println(err)
		return err
	}
	return nil
}

func (w *Deployment) CreateOperatorClusterRole() error {
	cmd := exec.Command("oc", "apply", "--namespace", config.Namespace, "-f", "deploy/role.yaml")
	output, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(output), "AlreadyExists") {
		fmt.Println(err)
		return err
	}
	return nil
}
