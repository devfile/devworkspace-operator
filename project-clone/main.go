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
	"io"
	"log"
	"os"
	"path"

	"github.com/devfile/devworkspace-operator/project-clone/internal"
	"github.com/devfile/devworkspace-operator/project-clone/internal/git"
	"github.com/devfile/devworkspace-operator/project-clone/internal/zip"
)

const (
	logFileName    = "project-clone-errors.log"
	tmpLogFilePath = "/tmp/" + logFileName
)

// TODO: Handle sparse checkout
// TODO: Add support for auth
func main() {
	f, err := os.Create(tmpLogFilePath)
	if err != nil {
		log.Printf("failed to open file %s for logging: %s", tmpLogFilePath, err)
	}
	mw := io.MultiWriter(os.Stdout, f)
	log.SetOutput(mw)

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
			copyLogFileToProjectsRoot()
			os.Exit(0)
		}
		if err != nil {
			log.Printf("Encountered error while setting up project %s: %s", project.Name, err)
			copyLogFileToProjectsRoot()
			os.Exit(0)
		}
	}
}

// copyLogFileToProjectsRoot copies the predefined log file into a persistent directory ($PROJECTS_ROOT)
// so that issues in setting up a devfile's projects are persisted beyond workspace restarts. Note that
// not all output from the project clone container is propagated to the log file. For example, the progress
// in cloning a project using the `git` binary only appears in stdout/stderr.
func copyLogFileToProjectsRoot() {
	infile, err := os.Open(tmpLogFilePath)
	if err != nil {
		log.Printf("Failed to open log file: %s", err)
	}
	defer infile.Close()
	outfile, err := os.Create(path.Join(internal.ProjectsRoot, logFileName))
	if err != nil {
		log.Printf("Failed to create log file: %s", err)
	}
	defer outfile.Close()

	if _, err := io.Copy(outfile, infile); err != nil {
		log.Printf("Failed to copy log file to $PROJECTS_ROOT: %s", err)
	}
}
