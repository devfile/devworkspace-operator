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

package restapis

import (
	"fmt"
	"testing"
)

func TestResolveClonePath(t *testing.T) {
	t.Run("When path isn't specified it defaults to the project name", testResolveClonePathFunc("", "mySampleProject", "/projects", "mySampleProject"))
	t.Run("When path is absolute then it defaults to the project name", testResolveClonePathFunc("/Documents/myProject", "mySampleProject", "/projects", "mySampleProject"))
	t.Run("If path escapes the project root then default to the project name", testResolveClonePathFunc("../../..", "mySampleProject", "/projects", "mySampleProject"))
	t.Run("If path escapes the project root then goes back in then then return the path", testResolveClonePathFunc("../projects/test", "mySampleProject", "/projects", "../projects/test"))
	t.Run("If path escapes the project root once then default to the project name", testResolveClonePathFunc("..", "mySampleProject", "/projects", "mySampleProject"))
	t.Run("If path is valid then return the path", testResolveClonePathFunc("src/github.com/devfile/devworkspace-operator", "mySampleProject", "/projects", "src/github.com/devfile/devworkspace-operator"))
}

func testResolveClonePathFunc(path string, projectName string, projectRoot string, expected string) func(*testing.T) {
	return func(t *testing.T) {
		actual := resolveClonePath(path, projectName, projectRoot)
		if actual != expected {
			t.Error(fmt.Sprintf("Expected %s but got %s instead", expected, actual))
		}
	}
}
