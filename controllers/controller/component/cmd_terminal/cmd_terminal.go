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

package cmd_terminal

import (
	"strings"

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha1"
)

const (
	CommandLineTerminalPublisherName    = "redhat-developer/web-terminal/"
	CommandLineTerminalDevPublisherName = "redhat-developer/web-terminal-dev/"
)

func ContainsCmdTerminalComponent(pluginComponents []devworkspace.Component) bool {
	for _, pc := range pluginComponents {
		if IsCommandLineTerminalPlugin(pc.Plugin) {
			return true
		}
	}
	return false
}

func IsCommandLineTerminalPlugin(p *devworkspace.PluginComponent) bool {
	if strings.HasPrefix(p.Id, CommandLineTerminalPublisherName) || strings.HasPrefix(p.Id, CommandLineTerminalDevPublisherName) {
		return true
	}
	return false
}
