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

// launch 'exec' oc command in the defined pod and container
func (w *K8sClient) ExecCommandInContainer(podName string, namespace, commandInContainer string) (output string, err error) {
	cmd := exec.Command("bash", "-c", fmt.Sprintf(
		"KUBECONFIG=%s oc exec %s -n %s -c restricted-access-container -- %s",
		w.kubeCfgFile,
		podName,
		namespace,
		commandInContainer))
	outBytes, err := cmd.CombinedOutput()
	return string(outBytes), err
}
