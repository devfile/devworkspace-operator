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

package flatten

import (
	"fmt"
	"strings"
)

const (
	EditorNameLabel          = "devworkspace.devfile.io/editor-name"
	EditorCompatibilityLabel = "devworkspace.devfile.io/editor-compatibility"
)

func checkPluginsCompatibility(resolveCtx *resolutionContextTree) error {
	editorNames, pluginReqEditors := processSubtreeCompatibility(resolveCtx)
	if len(editorNames) == 0 && len(pluginReqEditors) > 0 {
		var message []string
		for pluginEditor, pluginComponents := range pluginReqEditors {
			message = append(message,
				fmt.Sprintf("Component(s) [%s] depend on editor %s", strings.Join(pluginComponents, ", "), pluginEditor))
		}
		return fmt.Errorf("invalid plugins defined in devworkspace: no editor defined in workspace but %s", strings.Join(message, ". "))
	}
	if len(editorNames) > 1 {
		var editors []string
		for editorName, editorComponent := range editorNames {
			editors = append(editors, fmt.Sprintf("Component %s defines editor %s", editorComponent[0], editorName))
		}
		return fmt.Errorf("devworkspace defines multiple editors: %s", strings.Join(editors, ", "))
	}
	var editorName, editorComponentName string
	for name, componentNames := range editorNames {
		editorName = name
		if len(componentNames) > 1 {
			return fmt.Errorf("multiple components define the same editor: [%s]", strings.Join(componentNames, ", "))
		}
		editorComponentName = componentNames[0]
	}
	for pluginReqEditor, pluginComponents := range pluginReqEditors {
		if pluginReqEditor != editorName {
			return fmt.Errorf("devworkspace uses editor %s (defined in component %s) but plugins [%s] depend on editor %s",
				editorName, editorComponentName, strings.Join(pluginComponents, ", "), pluginReqEditor)
		}
	}

	return nil
}

func processSubtreeCompatibility(resolveCtx *resolutionContextTree) (editors, pluginReqEditors map[string][]string) {
	editors = map[string][]string{}
	pluginReqEditors = map[string][]string{}
	if resolveCtx.pluginMetadata != nil {
		if editor := resolveCtx.pluginMetadata[EditorNameLabel]; editor != "" {
			editors[editor] = append(editors[editor], resolveCtx.componentName)
		}
		if editorCompat := resolveCtx.pluginMetadata[EditorCompatibilityLabel]; editorCompat != "" {
			pluginReqEditors[editorCompat] = append(pluginReqEditors[editorCompat], resolveCtx.componentName)
		}
	}
	for _, plugin := range resolveCtx.plugins {
		subEditors, subPluginReqEditors := processSubtreeCompatibility(plugin)
		mergeMaps(subEditors, editors)
		mergeMaps(subPluginReqEditors, pluginReqEditors)
	}
	return editors, pluginReqEditors
}

func mergeMaps(from, into map[string][]string) {
	for k, v := range from {
		into[k] = append(into[k], v...)
	}
}
