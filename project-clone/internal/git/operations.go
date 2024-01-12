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
	"github.com/go-git/go-git/v5"
	gitConfig "github.com/go-git/go-git/v5/config"

	"github.com/devfile/devworkspace-operator/project-clone/internal"
	"github.com/devfile/devworkspace-operator/project-clone/internal/shell"
)

// CloneProject clones the project to path specified by projectPath
func CloneProject(project *dw.Project, projectPath string) error {
	log.Printf("Cloning project %s to %s", project.Name, projectPath)

	if len(project.Git.Remotes) == 0 {
		return fmt.Errorf("project does not define remotes")
	}

	var defaultRemoteName, defaultRemoteURL string
	if project.Git.CheckoutFrom != nil {
		defaultRemoteName = project.Git.CheckoutFrom.Remote
		if defaultRemoteName == "" {
			// omitting remote attribute is possible if there is a single remote
			if len(project.Git.Remotes) == 1 {
				for remoteName := range project.Git.Remotes {
					defaultRemoteName = remoteName
				}
			} else {
				// need to specify
				return fmt.Errorf("project checkoutFrom remote can't be omitted with multiple remotes")
			}
		}
		remoteURL, ok := project.Git.Remotes[defaultRemoteName]
		if !ok {
			return fmt.Errorf("project checkoutFrom refers to non-existing remote %s", defaultRemoteName)
		}
		defaultRemoteURL = remoteURL
	} else {
		if len(project.Git.Remotes) > 1 {
			return fmt.Errorf("project checkoutFrom field is required when a project defines multiple remotes")
		}
		for remoteName, remoteUrl := range project.Git.Remotes {
			defaultRemoteName, defaultRemoteURL = remoteName, remoteUrl
		}
	}

	if project.Attributes.Exists(internal.ProjectSparseCheckout) {
		if err := shell.GitSparseCloneProject(defaultRemoteURL, defaultRemoteName, projectPath); err != nil {
			return fmt.Errorf("failed to sparsely git clone from %s: %s", defaultRemoteURL, err)
		}
	} else {
		// Delegate to standard git binary because git.PlainClone takes a lot of memory for large repos
		err := shell.GitCloneProject(defaultRemoteURL, defaultRemoteName, projectPath)
		if err != nil {
			return fmt.Errorf("failed to git clone from %s: %s", defaultRemoteURL, err)
		}

	}

	log.Printf("Cloned project %s to %s", project.Name, projectPath)
	return nil
}

func SetupSparseCheckout(project *dw.Project, projectPath string) error {
	log.Printf("Setting up sparse checkout for project %s", project.Name)

	var err error
	sparseCheckoutDir := project.Attributes.GetString(internal.ProjectSparseCheckout, &err)
	if err != nil {
		return fmt.Errorf("failed to read %s attribute on project %s", internal.ProjectSparseCheckout, project.Name)
	}
	if sparseCheckoutDir == "" {
		return nil
	}
	if err := shell.GitSetupSparseCheckout(projectPath, sparseCheckoutDir); err != nil {
		return fmt.Errorf("error running sparse-checkout set: %w", err)
	}

	return nil
}

// SetupRemotes sets up a git remote in repo for each remote in project.Git.Remotes
func SetupRemotes(repo *git.Repository, project *dw.Project, projectPath string) error {
	log.Printf("Setting up remotes for project %s", project.Name)
	for remoteName, remoteUrl := range project.Git.Remotes {
		_, err := repo.CreateRemote(&gitConfig.RemoteConfig{
			Name: remoteName,
			URLs: []string{remoteUrl},
		})
		if err != nil && err != git.ErrRemoteExists {
			return fmt.Errorf("failed to add remote %s: %s", remoteName, err)
		}
		err = shell.GitFetchRemote(projectPath, remoteName)
		if err != nil {
			return fmt.Errorf("failed to fetch from remote %s: %s", remoteUrl, err)
		}
		log.Printf("Fetched remote %s at %s", remoteName, remoteUrl)
	}
	return nil
}

func SetupSubmodules(project *dw.Project, projectPath string) error {
	if _, err := os.Stat(path.Join(projectPath, ".gitmodules")); os.IsNotExist(err) {
		// No submodules; do nothing
		return nil
	}
	log.Printf("Initializing submodules for project %s", project.Name)
	if err := shell.GitInitSubmodules(projectPath); err != nil {
		return fmt.Errorf("git submodule update --init --recursive failed: %s", err)
	}
	return nil
}

// CheckoutReference sets the current HEAD in repo to point at the revision and remote referenced by checkoutFrom
func CheckoutReference(project *dw.Project, projectPath string) error {
	checkoutFrom := project.Git.CheckoutFrom
	if checkoutFrom == nil || checkoutFrom.Revision == "" {
		return nil
	}
	var defaultRemoteName string
	// multiple remotes error case is handled before at CloneProject step
	if checkoutFrom.Remote == "" && len(project.Git.Remotes) == 1 {
		for remoteName := range project.Git.Remotes {
			defaultRemoteName = remoteName
		}
	} else {
		defaultRemoteName = checkoutFrom.Remote
	}

	revision := checkoutFrom.Revision
	refType, err := shell.GitResolveReference(projectPath, defaultRemoteName, revision)
	if err != nil {
		return fmt.Errorf("failed to resolve git revision %s: %w", revision, err)
	}
	switch refType {
	case shell.GitRefLocalBranch:
		return checkoutLocalBranch(projectPath, revision, defaultRemoteName)
	case shell.GitRefRemoteBranch:
		return checkoutRemoteBranch(projectPath, revision, defaultRemoteName)
	case shell.GitRefTag:
		return checkoutTag(projectPath, revision)
	case shell.GitRefHash:
		return checkoutCommit(projectPath, revision)
	default:
		log.Printf("Could not find revision %s in repository, using default branch", checkoutFrom.Revision)
		return nil
	}
}

func checkoutLocalBranch(projectPath, branchName, remote string) error {
	log.Printf("Checking out local branch %s", branchName)
	if err := shell.GitCheckoutBranchLocal(projectPath, branchName); err != nil {
		return fmt.Errorf("failed to checkout branch %s: %s", branchName, err)
	}

	log.Printf("Setting tracking remote for branch %s to %s", branchName, remote)
	if err := shell.GitSetTrackingRemoteBranch(projectPath, branchName, remote); err != nil {
		return fmt.Errorf("failed to set tracking for branch %s: %w", branchName, err)
	}

	return nil
}

func checkoutRemoteBranch(projectPath, branchName, remote string) error {
	log.Printf("Checking out remote branch %s", branchName)

	if err := shell.GitCheckoutBranch(projectPath, branchName, remote); err != nil {
		return fmt.Errorf("failed to checkout branch %s: %s", branchName, err)
	}
	return nil
}

func checkoutTag(projectPath, tagName string) error {
	log.Printf("Checking out tag %s", tagName)

	if err := shell.GitCheckoutRef(projectPath, tagName); err != nil {
		return fmt.Errorf("failed to checkout tag %s: %s", tagName, err)
	}
	return nil
}

func checkoutCommit(projectPath, hash string) error {
	log.Printf("Checking out commit %s", hash)

	if err := shell.GitCheckoutRef(projectPath, hash); err != nil {
		return fmt.Errorf("failed to checkout commit %s: %s", hash, err)
	}
	return nil
}
