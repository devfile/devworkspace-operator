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
	"github.com/devfile/devworkspace-operator/project-clone/internal/zip"
)

// TODO: Handle sparse checkout
// TODO: Add support for auth
func main() {
	workspace, err := internal.ReadFlattenedDevWorkspace()
	if err != nil {
		log.Printf("Failed to read current DevWorkspace: %s", err)
		os.Exit(1)
	}
	for _, project := range workspace.Projects {
		log.Printf("Processing project %s", project.Name)
		var err error
		switch {
		case project.Git != nil:
			err = git.SetupGitProject(project)
		case project.Zip != nil:
			err = zip.SetupZipProject(project)
		default:
			log.Printf("Project does not specify Git or Zip source")
			os.Exit(1)
		}
		if err != nil {
			log.Printf("Encountered error while setting up project %s: %s", project.Name, err)
			os.Exit(1)
		}
	}
}
