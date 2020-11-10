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

package library

import (
	"fmt"

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha2"
)

func getCommandType(command devworkspace.Command) (devworkspace.CommandType, error) {
	err := command.Normalize()
	if err != nil {
		return "", err
	}
	return command.CommandType, nil
}

func getCommandsForKeys(key []string, commands []devworkspace.Command) ([]devworkspace.Command, error) {
	var resolvedCommands []devworkspace.Command

	for _, id := range key {
		resolvedCommand, err := getCommandByKey(id, commands)
		if err != nil {
			return nil, err
		}
		resolvedCommands = append(resolvedCommands, *resolvedCommand)
	}

	return resolvedCommands, nil
}

func getCommandByKey(key string, commands []devworkspace.Command) (*devworkspace.Command, error) {
	for _, command := range commands {
		commandKey := command.Key()
		if commandKey == key {
			return &command, nil
		}
	}
	return nil, fmt.Errorf("no command with ID %s is defined", key)
}

func commandListToComponentKeys(commands []devworkspace.Command) (map[string]bool, error) {
	componentKeys := map[string]bool{}
	for _, command := range commands {
		commandType, err := getCommandType(command)
		if err != nil {
			return nil, err
		}
		switch commandType {
		case devworkspace.ApplyCommandType:
			componentKeys[command.Apply.Component] = true
		case devworkspace.ExecCommandType:
			// TODO: This will require special handling (how do we handle prestart exec?)
			componentKeys[command.Exec.Component] = true
		case devworkspace.CompositeCommandType:
			// TODO: Handle composite commands: what if an init command is composite and refers to other commands
		default: // Ignore
		}
	}
	return componentKeys, nil
}

func removeCommandsByKeys(keys []string, commands []devworkspace.Command) ([]devworkspace.Command, error) {
	var filtered []devworkspace.Command
	for _, command := range commands {
		key := command.Key()
		if !listContains(key, keys) {
			filtered = append(filtered, command)
		}
	}
	return filtered, nil
}
