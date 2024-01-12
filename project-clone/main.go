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

package main

import (
	"crypto/tls"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"syscall"

	projectslib "github.com/devfile/devworkspace-operator/pkg/library/projects"
	"github.com/devfile/devworkspace-operator/project-clone/internal"
	"github.com/devfile/devworkspace-operator/project-clone/internal/bootstrap"
	"github.com/devfile/devworkspace-operator/project-clone/internal/git"
	"github.com/devfile/devworkspace-operator/project-clone/internal/zip"
	gitclient "github.com/go-git/go-git/v5/plumbing/transport/client"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

const (
	logFileName    = "project-clone-errors.log"
	tmpLogFilePath = "/tmp/" + logFileName
)

func main() {
	f, err := os.Create(tmpLogFilePath)
	if err != nil {
		log.Printf("failed to open file %s for logging: %s", tmpLogFilePath, err)
	}
	mw := io.MultiWriter(os.Stdout, f)
	log.SetOutput(mw)

	// Clean up temp dir on exit
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		if err := os.RemoveAll(internal.CloneTmpDir); err != nil {
			log.Printf("Encountered error cleaning up temporary directory: %s", err)
		}
		os.Exit(0)
	}()
	defer os.RemoveAll(internal.CloneTmpDir)

	workspace, err := internal.ReadFlattenedDevWorkspace()
	if err != nil {
		log.Printf("Failed to read current DevWorkspace: %s", err)
		os.Exit(1)
	}

	projects := workspace.Projects
	projects = append(projects, workspace.DependentProjects...)

	starterProject, err := projectslib.GetStarterProject(workspace)
	if err != nil {
		log.Printf("Encountered error while processing starterProjects: %s", err)
	} else if starterProject != nil {
		projects = append(projects, internal.StarterProjectToRegularProject(starterProject))
	}

	var httpClient *http.Client
	certs, warnings, err := internal.GetAdditionalCerts()
	for _, warning := range warnings {
		log.Printf("Warning while reading additional certificates: %s", warning)
	}
	if err != nil || certs == nil {
		log.Printf("Failed to read additional certificates: %s", err)
		log.Printf("Using default system certificate pool")
		httpClient = http.DefaultClient
	} else {
		httpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs: certs,
				},
			},
		}
		gitclient.InstallProtocol("https", githttp.NewClient(httpClient))
	}

	encounteredError := false
	for _, project := range projects {
		log.Printf("Processing project %s", project.Name)
		var err error
		switch {
		case project.Git != nil:
			err = git.SetupGitProject(project)
		case project.Zip != nil:
			err = zip.SetupZipProject(project, httpClient)
		default:
			log.Printf("Project does not specify Git or Zip source")
			copyLogFileToProjectsRoot()
			os.Exit(0)
		}
		if err != nil {
			log.Printf("Encountered error while setting up project %s: %s", project.Name, err)
			encounteredError = true
		}
	}
	if encounteredError {
		copyLogFileToProjectsRoot()
		os.Exit(0)
	}

	needBootstrap, err := bootstrap.NeedsBootstrap(workspace)
	if err != nil {
		log.Printf("Encountered error reading DevWorkspace attributes: %s", err)
		copyLogFileToProjectsRoot()
	} else if needBootstrap {
		if err := bootstrap.BootstrapWorkspace(workspace); err != nil {
			log.Printf("Encountered error setting up DevWorkspace from devfile: %s", err)
			copyLogFileToProjectsRoot()
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
