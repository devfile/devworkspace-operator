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

package main

import (
	"log"
	"os"

	"github.com/devfile/devworkspace-operator/project-clone/internal"
	"github.com/devfile/devworkspace-operator/project-clone/internal/git"
)

// TODO: Handle projects specifying a zip file instead of git
// TODO: Handle sparse checkout
// TODO: Add support for auth
func main() {
	workspace, err := internal.ReadFlattenedDevWorkspace()
	if err != nil {
		log.Printf("Failed to read current DevWorkspace: %s", err)
		os.Exit(1)
	}
	for _, project := range workspace.Projects {
		var err error
		switch {
		case project.Git != nil:
			err = git.SetupGitProject(project)
		default:
			log.Printf("Unsupported project type")
			os.Exit(1)
		}
		if err != nil {
			log.Printf("Encountered error while setting up project %s: %s", project.Name, err)
		}
	}
}
