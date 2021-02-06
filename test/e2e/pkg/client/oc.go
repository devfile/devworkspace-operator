//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package client

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

func (w *K8sClient) OcApplyWorkspace(namespace string, filePath string) (commandResult string, err error) {
	cmd := exec.Command("bash", "-c", fmt.Sprintf(
		"KUBECONFIG=%s oc apply --namespace %s -f %s",
		w.kubeCfgFile,
		namespace,
		filePath))
	outBytes, err := cmd.CombinedOutput()
	output := string(outBytes)

	if strings.Contains(output, "failed calling webhook") {
		log.Print("Seems DevWorkspace Webhook Server is not ready yet. Will retry in 2 seconds. Cause: " + output)
		time.Sleep(2 * time.Second)
		return w.OcApplyWorkspace(namespace, filePath)
	}
	if err != nil && !strings.Contains(output, "AlreadyExists") {
		return output, err
	}

	return output, nil
}

//launch 'exec' oc command in the defined pod and container
func (w *K8sClient) ExecCommandInContainer(podName string, namespace, commandInContainer string) (output string, err error) {
	cmd := exec.Command("bash", "-c", fmt.Sprintf(
		"KUBECONFIG=%s oc exec %s -n %s -c dev -- %s",
		w.kubeCfgFile,
		podName,
		namespace,
		commandInContainer))
	outBytes, err := cmd.CombinedOutput()
	return string(outBytes), err
}
