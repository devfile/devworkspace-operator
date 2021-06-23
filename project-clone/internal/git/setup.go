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
	"fmt"
	"log"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"

	"github.com/devfile/devworkspace-operator/project-clone/internal"
)

func SetupGitProject(project v1alpha2.Project) error {
	needClone, needRemotes, err := internal.CheckProjectState(&project)
	if err != nil {
		return fmt.Errorf("failed to check state of repo on disk: %s", err)
	}
	if needClone {
		err := CloneProject(&project)
		if err != nil {
			return fmt.Errorf("failed to clone project: %s", err)
		}
		repo, err := internal.OpenRepo(&project)
		if err != nil {
			return fmt.Errorf("failed to open existing project in filesystem: %s", err)
		} else if repo == nil {
			return fmt.Errorf("unexpected error while setting up remotes for project: git repository not present")
		}
		if err := SetupRemotes(repo, &project); err != nil {
			return fmt.Errorf("failed to set up remotes for project: %s", err)
		}
		if err := CheckoutReference(repo, &project); err != nil {
			return fmt.Errorf("failed to checkout revision: %s", err)
		}
	} else if needRemotes {
		repo, err := internal.OpenRepo(&project)
		if err != nil {
			return fmt.Errorf("failed to open existing project in filesystem: %s", err)
		} else if repo == nil {
			return fmt.Errorf("unexpected error while setting up remotes for project: git repository not present")
		}
		if err := SetupRemotes(repo, &project); err != nil {
			return fmt.Errorf("failed to set up remotes for project: %s", err)
		}
	} else {
		log.Printf("Project '%s' is already cloned and has all remotes configured", project.Name)
	}
	return nil
}
