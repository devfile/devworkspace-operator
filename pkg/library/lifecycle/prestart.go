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
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

// GetInitContainers partitions the components in a devfile's flattened spec into initContainer and non-initContainer lists
// based off devfile lifecycle bindings and commands. Note that a component can appear in both lists, if e.g. it referred to
// in a preStart command and in a regular command.
func GetInitContainers(devfile dw.DevWorkspaceTemplateSpecContent) (initContainers, mainComponents []dw.Component, err error) {
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
		componentID := component.Key()
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

	// This is necessary because if there is another initcontainer that runs before it,
	// an automount resource can mount within /home/user and cause stow conflicts
	// in init-persistent-home since the automount resource would be saved in the PVC.
	initContainers = setHomeInitContainerToFront(initContainers)
	return initContainers, mainComponents, nil
}

// Takes a slice of components which are intended to run as initContainers and moves the
// init-persistent-home component to the start of the slice.
func setHomeInitContainerToFront(initContainers []dw.Component) []dw.Component {
	for i, container := range initContainers {
		if container.Name == constants.HomeInitComponentName {
			initContainers = append(initContainers[:i], initContainers[i+1:]...)
			return append([]dw.Component{container}, initContainers...)
		}
	}
	return initContainers
}

func checkPreStartEventCommandsValidity(initCommands []dw.Command) error {
	for _, cmd := range initCommands {
		commandType, err := getCommandType(cmd)
		if err != nil {
			return err
		}
		switch commandType {
		case dw.ApplyCommandType:
			continue
		default:
			// How a prestart exec command should be implemented is undefined currently, so we reject it.
			// Other types of commands cannot be included in the preStart event hook.
			return fmt.Errorf("only apply-type commands are supported in the prestart lifecycle binding")
		}
	}
	return nil
}
