//
// Copyright (c) 2019-2024 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
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
