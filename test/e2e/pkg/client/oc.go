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

package client

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/devfile/devworkspace-operator/test/e2e/pkg/config"
)

func (w *K8sClient) OcApplyWorkspace(filePath string) (err error) {
	cmd := exec.Command("oc", "apply", "--namespace", config.Namespace, "-f", filePath)
	outBytes, err := cmd.CombinedOutput()
	output := string(outBytes)
	if strings.Contains(output, "failed calling webhook") {
		fmt.Println("Seems DevWorkspace Webhook Server is not ready yet. Will retry in 2 seconds. Cause: " + output)
		time.Sleep(2 * time.Second)
		return w.OcApplyWorkspace(filePath)
	}
	if err != nil && !strings.Contains(output, "AlreadyExists") {
		fmt.Println(err)
	}
	return err
}
