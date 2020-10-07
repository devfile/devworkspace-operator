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

package plugin_patch

import (
	"strings"

	"github.com/eclipse/che-plugin-broker/model"
)

// PublicAccessPatch patches plugin's configuration to make endpoints publicly available
// Plugins are configured to listen to only localhost only since they don't provide any authentication
// and their endpoint is supposed to be accessed though some proxy that provides authentication/authorization.
// Since authentication is not solved for devworkspace endpoints, it's the workaround to make endpoints available somehow for testing
func PublicAccessPatch(plugin *model.ChePlugin) {
	if plugin.Name == "che-machine-exec-plugin" {
		for _, value := range plugin.Containers {
			for i, command := range value.Command {
				if strings.Contains(command, "127.0.0.1:4444") {
					value.Command[i] = "0.0.0.0:4444"
				}
			}
		}
	}

	if plugin.Name == "che-theia" {
		for _, container := range plugin.Containers {
			for i, env := range container.Env {
				if env.Name == "THEIA_HOST" {
					container.Env[i].Value = "0.0.0.0"
				}
			}
		}
	}
}
