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

package flatten

import (
	"fmt"
	"reflect"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

// resolutionContextTree is a recursive structure representing information about the devworkspace that is
// lost when flattening
type resolutionContextTree struct {
	componentName   string
	importReference dw.ImportReference
	plugins         []*resolutionContextTree
	parentNode      *resolutionContextTree
}

func (t *resolutionContextTree) addPlugin(name string, plugin *dw.PluginComponent) *resolutionContextTree {
	newNode := &resolutionContextTree{
		componentName:   name,
		importReference: plugin.ImportReference,
		parentNode:      t,
	}
	t.plugins = append(t.plugins, newNode)
	return newNode
}

func (t *resolutionContextTree) hasCycle() error {
	var seenRefs []dw.ImportReference
	currNode := t
	for currNode.parentNode != nil {
		for _, seenRef := range seenRefs {
			if reflect.DeepEqual(seenRef, currNode.importReference) {
				return fmt.Errorf("DevWorkspace has an cycle in references: %s", formatImportCycle(t))
			}
		}
		seenRefs = append(seenRefs, currNode.importReference)
		currNode = currNode.parentNode
	}
	return nil
}

func formatImportCycle(end *resolutionContextTree) string {
	cycle := fmt.Sprintf("%s", end.componentName)
	for end.parentNode != nil {
		end = end.parentNode
		if end.parentNode == nil {
			end.componentName = "devworkspace"
		}
		cycle = fmt.Sprintf("%s -> %s", end.componentName, cycle)
	}
	return cycle
}

// getSourceForComponent returns the 'original' name for a component in a flattened DevWorkspace. Given a component, it
// returns the name of the plugin component that imported it if the component came via a plugin, and the actual
// component name otherwise.
//
// The purpose of this function is mainly to enable providing better messages to end-users, as a component name may
// not match the name of the plugin in the original DevWorkspace.
func getSourceForComponent(component dw.Component) string {
	if component.Attributes.Exists(constants.PluginSourceAttribute) {
		var err error
		componentName := component.Attributes.GetString(constants.PluginSourceAttribute, &err)
		if err == nil {
			return componentName
		}
	}
	return component.Name
}
