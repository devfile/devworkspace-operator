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
	"github.com/eclipse/che-plugin-broker/model"
)

// AddMachineNameEnv Adds the environment variable CHE_MACHINE_NAME to a plugin, in order
// to allow che-machine-exec to find the container within the workspace pod.
//
// Note this matching is fragile, as it is unclear how to handle aliases on components correctly.
func AddMachineNameEnv(plugin *model.ChePlugin) {
	for i, container := range plugin.Containers {
		plugin.Containers[i].Env = append(container.Env, model.EnvVar{
			Name:  "CHE_MACHINE_NAME",
			Value: container.Name,
		})
	}
}
