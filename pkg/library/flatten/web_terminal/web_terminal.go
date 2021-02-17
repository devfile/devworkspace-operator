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

package web_terminal

import (
	"fmt"
	"strings"

	devworkspace "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/config"
)

const webTerminalPluginName = "web-terminal"

var webTerminalPublishers = []string{
	"redhat-developer/web-terminal/",
	"redhat-developer/web-terminal-dev/",
}

func IsWebTerminalDevWorkspace(workspace *devworkspace.DevWorkspaceTemplateSpec) bool {
	for _, component := range workspace.Components {
		if component.Plugin != nil && pluginIsWebTerminal(component.Plugin) {
			return true
		}
	}
	return false
}

func AddDefaultContainerIfNeeded(workspace *devworkspace.DevWorkspaceTemplateSpec) error {
	if !IsWebTerminalDevWorkspace(workspace) || hasContainerComponent(workspace) {
		return nil
	}
	defaultComponent, err := config.ControllerCfg.GetDefaultTerminalDockerimage()
	if err != nil {
		return fmt.Errorf("failed to get default container component for web terminal: %w", err)
	}
	workspace.Components = append(workspace.Components, *defaultComponent)
	return nil
}

func pluginIsWebTerminal(plugin *devworkspace.PluginComponent) bool {
	// Check that ID matches web terminal publishers
	for _, publisher := range webTerminalPublishers {
		if strings.HasPrefix(plugin.Id, publisher) {
			return true
		}
	}
	if plugin.Kubernetes != nil && plugin.Kubernetes.Name == webTerminalPluginName {
		return true
	}
	return false
}

func hasContainerComponent(workspace *devworkspace.DevWorkspaceTemplateSpec) bool {
	for _, component := range workspace.Components {
		if component.Container != nil {
			return true
		}
	}
	return false
}
