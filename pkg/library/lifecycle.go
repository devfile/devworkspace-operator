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
	"github.com/devfile/api/pkg/apis/workspaces/v1alpha1"
)

func GetInitContainers(devfile v1alpha1.DevWorkspaceTemplateSpecContent) (initContainers, mainComponents []v1alpha1.Component, err error) {
	components := devfile.Components
	commands := devfile.Commands
	events := devfile.Events
	if events == nil || commands == nil {
		// All components are
		return nil, components, nil
	}

	initCommands, err := getCommandsForIds(events.PreStart, commands)
	if err != nil {
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
