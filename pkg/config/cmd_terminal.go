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

package config

import (
	"fmt"

	"github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	"gopkg.in/yaml.v2"
)

const (
	// property name for value with yaml for default dockerimage component
	// that should be provisioned if devfile DOES have che-incubator/command-line-terminal cheEditor
	// and DOES NOT have any dockerimage component
	defaultTerminalDockerimageProperty = "che.workspace.default_dockerimage.che-incubator.command-line-terminal"
)

var (
	defaultTerminalDockerimage = &v1alpha1.ComponentSpec{
		Type:        "dockerimage",
		MemoryLimit: "256Mi",
		Alias:       "dev",
		Image:       "registry.redhat.io/codeready-workspaces/plugin-openshift-rhel8:2.1",
		Args:        []string{"tail", "-f", "/dev/null"},
		Env: []v1alpha1.Env{
			{
				Name:  "PS1",
				Value: "\\[\\e[34m\\]>\\[\\e[m\\]\\[\\e[33m\\]>\\[\\e[m\\]",
			},
		},
	}
)

func (wc *ControllerConfig) GetDefaultTerminalDockerimage() (*v1alpha1.ComponentSpec, error) {
	defaultDockerimageYaml := wc.GetProperty(defaultTerminalDockerimageProperty)
	if defaultDockerimageYaml == nil {
		return defaultTerminalDockerimage.DeepCopy(), nil
	}

	var dockerimage v1alpha1.ComponentSpec
	if err := yaml.Unmarshal([]byte(*defaultDockerimageYaml), &dockerimage); err != nil {
		return nil, fmt.Errorf(
			"%s is configure with invalid dockerimage component. Error: %s", defaultTerminalDockerimageProperty, err)
	}

	return &dockerimage, nil
}
