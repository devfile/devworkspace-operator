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

package git

import (
	"fmt"
	"log"
	"os"
	"path"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	projectslib "github.com/devfile/devworkspace-operator/pkg/library/projects"

	"github.com/devfile/devworkspace-operator/project-clone/internal"
)

func SetupGitProject(project dw.Project) error {
	needClone, needRemotes, err := internal.CheckProjectState(&project)
	if err != nil {
		return fmt.Errorf("failed to check state of repo on disk: %s", err)
	}
	if needClone {
		return doInitialGitClone(&project)
	} else if needRemotes {
		return setupRemotesForExistingProject(&project)
	} else {
		log.Printf("Project '%s' is already cloned and has all remotes configured", project.Name)
		return nil
	}
}

func doInitialGitClone(project *dw.Project) error {
	// Clone into a temp dir and then move set up project to PROJECTS_ROOT to try and make clone atomic in case
	// project-clone container is terminated
	tmpClonePath := path.Join(internal.CloneTmpDir, projectslib.GetClonePath(project))
	err := CloneProject(project, tmpClonePath)
	if err != nil {
		return fmt.Errorf("failed to clone project: %s", err)
	}

	if project.Attributes.Exists(internal.ProjectSparseCheckout) {
		if err := SetupSparseCheckout(project, tmpClonePath); err != nil {
			return fmt.Errorf("failed to set up sparse checkout on project %s: %w", project.Name, err)
		}
	}

	repo, err := internal.OpenRepo(tmpClonePath)
	if err != nil {
		return fmt.Errorf("failed to open existing project in filesystem: %s", err)
	} else if repo == nil {
		return fmt.Errorf("unexpected error while setting up remotes for project: git repository not present")
	}

	if err := SetupRemotes(repo, project, tmpClonePath); err != nil {
		return fmt.Errorf("failed to set up remotes for project: %s", err)
	}

	if err := CheckoutReference(project, tmpClonePath); err != nil {
		return fmt.Errorf("failed to checkout revision: %s", err)
	}

	if err := SetupSubmodules(project, tmpClonePath); err != nil {
		log.Printf("Failed to set up submodules in project: %s", err)
	}

	if err := copyProjectFromTmpDir(project, tmpClonePath); err != nil {
		return err
	}

	return nil
}

func setupRemotesForExistingProject(project *dw.Project) error {
	projectPath := path.Join(internal.ProjectsRoot, projectslib.GetClonePath(project))
	repo, err := internal.OpenRepo(projectPath)
	if err != nil {
		return fmt.Errorf("failed to open existing project in filesystem: %s", err)
	} else if repo == nil {
		return fmt.Errorf("unexpected error while setting up remotes for project: git repository not present")
	}
	if err := SetupRemotes(repo, project, projectPath); err != nil {
		return fmt.Errorf("failed to set up remotes for project: %s", err)
	}
	return nil
}

func copyProjectFromTmpDir(project *dw.Project, tmpClonePath string) error {
	if project.Attributes.Exists(internal.ProjectSubDir) {
		// Only want one directory from the project
		var err error
		subDirSubPath := project.Attributes.GetString(internal.ProjectSubDir, &err)
		if err != nil {
			return fmt.Errorf("failed to process subDir on project: %w", err)
		}
		subDirPath := path.Join(tmpClonePath, subDirSubPath)
		projectPath := path.Join(internal.ProjectsRoot, projectslib.GetClonePath(project))
		log.Printf("Moving subdirectory %s in project %s from temporary directory to %s", subDirSubPath, project.Name, projectPath)
		if err := os.Rename(subDirPath, projectPath); err != nil {
			return fmt.Errorf("failed to move subdirectory of cloned project to %s: %w", internal.ProjectsRoot, err)
		}
	} else {
		projectPath := path.Join(internal.ProjectsRoot, projectslib.GetClonePath(project))
		log.Printf("Moving cloned project %s from temporary directory %s to %s", project.Name, tmpClonePath, projectPath)
		if err := os.Rename(tmpClonePath, projectPath); err != nil {
			return fmt.Errorf("failed to move cloned project to %s: %w", internal.ProjectsRoot, err)
		}
	}
	return nil
}
