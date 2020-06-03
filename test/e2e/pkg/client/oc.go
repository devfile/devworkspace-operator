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
	"github.com/che-incubator/che-workspace-operator/test/e2e/pkg/config"
	"os/exec"
	"strings"
)

func (w *K8sClient) OcApply(filePath string) (err error) {
	cmd := exec.Command("oc", "apply", "--namespace", config.Namespace, "-f", filePath)
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil && !strings.Contains(string(output), "AlreadyExists") {
		fmt.Println(err)
	}
	return err
}
