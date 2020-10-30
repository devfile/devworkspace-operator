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

	"github.com/devfile/api/pkg/apis/workspaces/v1alpha1"
)

// GetInitContainers partitions the components in a devfile's flattened spec into initContainer and non-initContainer lists
// based off devfile lifecycle bindings and commands. Note that a component can appear in both lists, if e.g. it referred to
// in a preStart command and in a regular command.
func GetInitContainers(devfile v1alpha1.DevWorkspaceTemplateSpecContent) (initContainers, mainComponents []v1alpha1.Component, err error) {
	components := devfile.Components
	commands := devfile.Commands
	events := devfile.Events
	if events == nil || commands == nil {
		// All components should be run in the main deployment
		return nil, components, nil
	}

	initCommands, err := getCommandsForKeys(events.PreStart, commands)
	if err != nil {
		return nil, nil, err
	}
	// Check that commands in PreStart lifecycle binding are supported
	if err = checkPreStartEventCommandsValidity(initCommands); err != nil {
		return nil, nil, err
	}
	initComponentKeys, err := commandListToComponentKeys(initCommands)
	if err != nil {
		return nil, nil, err
	}

	// Need to also consider components that are *both* init containers and in the main deployment
	// Example: component is referenced in both a prestart event and a regular, non-prestart command
	// TODO: Figure out details of handling postStop commands, since they should not be included in main deployment
	nonInitCommands, err := removeCommandsByKeys(events.PreStart, commands)
	if err != nil {
		return nil, nil, err
	}
	mainComponentKeys, err := commandListToComponentKeys(nonInitCommands)
	if err != nil {
		return nil, nil, err
	}

	for _, component := range components {
		componentID, err := component.Key()
		if err != nil {
			return nil, nil, err
		}
		if initComponentKeys[componentID] {
			initContainers = append(initContainers, component)
			if mainComponentKeys[componentID] {
				// Component is *also* a main component.
				mainComponents = append(mainComponents, component)
			}
		} else {
			mainComponents = append(mainComponents, component)
		}
	}

	return initContainers, mainComponents, nil
}

func checkPreStartEventCommandsValidity(initCommands []v1alpha1.Command) error {
	for _, cmd := range initCommands {
		commandType, err := getCommandType(cmd)
		if err != nil {
			return err
		}
		switch commandType {
		case v1alpha1.ApplyCommandType:
			continue
		default:
			// How a prestart exec command should be implemented is undefined currently, so we reject it.
			// Other types of commands cannot be included in the preStart event hook.
			return fmt.Errorf("only apply-type commands are supported in the prestart lifecycle binding")
		}
	}
	return nil
}
