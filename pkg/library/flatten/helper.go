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

package flatten

import (
	"fmt"
	"reflect"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
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
