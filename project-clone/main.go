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
	"fmt"
	"io/ioutil"
	"log"
	"os"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"sigs.k8s.io/yaml"

	"github.com/devfile/devworkspace-operator/pkg/provision/metadata"
	"github.com/devfile/devworkspace-operator/project-clone/internal"
)

// TODO: Handle projects specifying a zip file instead of git
// TODO: Handle sparse checkout
// TODO: Add support for auth
func main() {
	workspace, err := readFlattenedDevWorkspace()
	if err != nil {
		log.Printf("Failed to read current DevWorkspace: %s", err)
		os.Exit(1)
	}
	for _, project := range workspace.Projects {
		if project.Git == nil {
			continue
		}
		needClone, needRemotes, err := internal.CheckProjectState(&project)
		if err != nil {
			log.Printf("Failed to process project %s: %s", project.Name, err)
			os.Exit(1)
		}
		if needClone {
			repo, err := internal.CloneProject(&project)
			if err != nil {
				log.Printf("failed to clone project %s: %s", project.Name, err)
				os.Exit(1)
			}
			if err := internal.SetupRemotes(repo, &project); err != nil {
				log.Printf("Failed to set up remotes for project %s: %s", project.Name, err)
				os.Exit(1)
			}
			if err := internal.CheckoutReference(repo, project.Git.CheckoutFrom); err != nil {
				log.Printf("Failed to checkout revision for project %s: %s", project.Name, err)
				os.Exit(1)
			}
		} else if needRemotes {
			repo, err := internal.OpenRepo(&project)
			if err != nil {
				log.Printf("Failed to open existing project %s: %s", project.Name, err)
				os.Exit(1)
			} else if repo == nil {
				log.Printf("Unexpected error while setting up remotes for project %s: git repository not present", project.Name)
				os.Exit(1)
			}
			if err := internal.SetupRemotes(repo, &project); err != nil {
				log.Printf("Failed to set up remotes for project %s: %s", project.Name, err)
				os.Exit(1)
			}
		} else {
			log.Printf("Project '%s' is already cloned and has all remotes configured", project.Name)
		}
	}
}

func readFlattenedDevWorkspace() (*dw.DevWorkspaceTemplateSpec, error) {
	flattenedDevWorkspacePath := os.Getenv(metadata.FlattenedDevfileMountPathEnvVar)
	if flattenedDevWorkspacePath == "" {
		return nil, fmt.Errorf("required environment variable %s is unset", metadata.FlattenedDevfileMountPathEnvVar)
	}

	fileBytes, err := ioutil.ReadFile(flattenedDevWorkspacePath)
	if err != nil {
		return nil, fmt.Errorf("error reading current DevWorkspace YAML: %s", err)
	}

	dwts := &dw.DevWorkspaceTemplateSpec{}
	if err := yaml.Unmarshal(fileBytes, dwts); err != nil {
		return nil, fmt.Errorf("error unmarshalling DevWorkspace YAML: %s", err)
	}

	log.Printf("Read DevWorkspace at %s", flattenedDevWorkspacePath)
	return dwts, nil
}
