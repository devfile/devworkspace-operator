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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/devfile/devworkspace-operator/test/e2e/pkg/config"
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

func (w *Deployment) CreateAdditionalControllerResources() error {
	//sed "s/\${NAMESPACE}/che/g" <<< cat *.yaml | oc apply -f -
	cmd := exec.Command(
		"bash", "-c",
		"sed 's/\\${NAMESPACE}/"+config.Namespace+"/g' <<< "+
			"cat deploy/*.yaml | "+
			"oc apply --namespace "+config.Namespace+" -f -")
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

func (w *Deployment) CustomResourceDefinitions() error {
	devWorkspaceCRD := exec.Command("oc", "apply", "-f", "devworkspace-crds/deploy/crds")
	output, err := devWorkspaceCRD.CombinedOutput()
	if err != nil && !strings.Contains(string(output), "AlreadyExists") {
		fmt.Println(err)
		return err
	}

	eclipseCRD := exec.Command("oc", "apply", "-f", "deploy/crds")
	output, err = eclipseCRD.CombinedOutput()
	if err != nil && !strings.Contains(string(output), "AlreadyExists") {
		fmt.Println(err)
		return err
	}

	return nil
}
