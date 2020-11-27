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

	"github.com/devfile/devworkspace-operator/internal/images"

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha2"

	"sigs.k8s.io/yaml"
)

const (
	// property name for value with yaml for default dockerimage component
	// that should be provisioned if devfile DOES have redhat-developer/web-terminal plugin
	// and DOES NOT have any dockerimage component
	defaultTerminalDockerimageProperty = "devworkspace.default_dockerimage.redhat-developer.web-terminal"
)

func (wc *ControllerConfig) GetDefaultTerminalDockerimage() (*devworkspace.Component, error) {
	defaultDockerimageYaml := wc.GetProperty(defaultTerminalDockerimageProperty)
	if defaultDockerimageYaml == nil {
		webTerminalImage := images.GetWebTerminalToolingImage()
		if webTerminalImage == "" {
			return nil, fmt.Errorf("cannot determine default image for web terminal: environment variable is unset")
		}
		defaultTerminalDockerimage := &devworkspace.Component{}
		defaultTerminalDockerimage.Name = "dev"
		defaultTerminalDockerimage.Container = &devworkspace.ContainerComponent{
			Container: devworkspace.Container{
				Image:       webTerminalImage,
				Args:        []string{"tail", "-f", "/dev/null"},
				MemoryLimit: "256Mi",
				Env: []devworkspace.EnvVar{
					{
						Name:  "PS1",
						Value: `\[\e[34m\]>\[\e[m\]\[\e[33m\]>\[\e[m\]`,
					},
				},
				// Must be set as it is defaulted in ContainerComponent. Otherwise
				// spec and cluster objects will be different.
				SourceMapping: "/projects",
			},
		}
		return defaultTerminalDockerimage, nil
	}

	var dockerimage devworkspace.Component
	if err := yaml.Unmarshal([]byte(*defaultDockerimageYaml), &dockerimage); err != nil {
		return nil, fmt.Errorf(
			"%s is configured with invalid container component. Error: %s", defaultTerminalDockerimageProperty, err)
	}

	return &dockerimage, nil
}
