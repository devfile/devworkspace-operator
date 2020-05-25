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

	"github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
)

const (
	CommandLineTerminalPublisherName = "che-incubator/command-line-terminal/"
)

func ContainsCmdTerminalComponent(plugins []v1alpha1.ComponentSpec) bool {
	for _, p := range plugins {
		if IsCommandLineTerminalPlugin(p) {
			return true
		}
	}
	return false
}

func IsCommandLineTerminalPlugin(p v1alpha1.ComponentSpec) bool {
	if strings.HasPrefix(p.Id, CommandLineTerminalPublisherName) {
		return true
	}
	return false
}
