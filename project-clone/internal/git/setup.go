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

package git

import (
	"log"
	"os"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"

	"github.com/devfile/devworkspace-operator/project-clone/internal"
)

func SetupGitProject(project v1alpha2.Project) error {
	needClone, needRemotes, err := internal.CheckProjectState(&project)
	if err != nil {
		log.Printf("Failed to process project %s: %s", project.Name, err)
		os.Exit(1)
	}
	if needClone {
		repo, err := CloneProject(&project)
		if err != nil {
			log.Printf("failed to clone project %s: %s", project.Name, err)
			os.Exit(1)
		}
		if err := SetupRemotes(repo, &project); err != nil {
			log.Printf("Failed to set up remotes for project %s: %s", project.Name, err)
			os.Exit(1)
		}
		if err := CheckoutReference(repo, &project); err != nil {
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
		if err := SetupRemotes(repo, &project); err != nil {
			log.Printf("Failed to set up remotes for project %s: %s", project.Name, err)
			os.Exit(1)
		}
	} else {
		log.Printf("Project '%s' is already cloned and has all remotes configured", project.Name)
	}
	return nil
}
