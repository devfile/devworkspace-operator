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

package lifecycle

import (
	"fmt"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
)

func getCommandType(command dw.Command) (dw.CommandType, error) {
	err := command.Normalize()
	if err != nil {
		return "", err
	}
	return command.CommandType, nil
}

func getCommandsForKeys(key []string, commands []dw.Command) ([]dw.Command, error) {
	var resolvedCommands []dw.Command

	for _, id := range key {
		resolvedCommand, err := getCommandByKey(id, commands)
		if err != nil {
			return nil, err
		}
		resolvedCommands = append(resolvedCommands, *resolvedCommand)
	}

	return resolvedCommands, nil
}

func getCommandByKey(key string, commands []dw.Command) (*dw.Command, error) {
	for _, command := range commands {
		commandKey := command.Key()
		if commandKey == key {
			return &command, nil
		}
	}
	return nil, fmt.Errorf("no command with ID %s is defined", key)
}

func commandListToComponentKeys(commands []dw.Command) (map[string]bool, error) {
	componentKeys := map[string]bool{}
	for _, command := range commands {
		commandType, err := getCommandType(command)
		if err != nil {
			return nil, err
		}
		switch commandType {
		case dw.ApplyCommandType:
			componentKeys[command.Apply.Component] = true
		case dw.ExecCommandType:
			// TODO: This will require special handling (how do we handle prestart exec?)
			componentKeys[command.Exec.Component] = true
		case dw.CompositeCommandType:
			// TODO: Handle composite commands: what if an init command is composite and refers to other commands
		default: // Ignore
		}
	}
	return componentKeys, nil
}

func removeCommandsByKeys(keys []string, commands []dw.Command) ([]dw.Command, error) {
	var filtered []dw.Command
	for _, command := range commands {
		key := command.Key()
		if !listContains(key, keys) {
			filtered = append(filtered, command)
		}
	}
	return filtered, nil
}
