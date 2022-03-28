//
// Copyright (c) 2019-2022 Red Hat, Inc.
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

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/go-git/go-git/v5"
	gitConfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"

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

	// Delegate to standard git binary because git.PlainClone takes a lot of memory for large repos
	err := shell.GitCloneProject(defaultRemoteURL, defaultRemoteName, projectPath)
	if err != nil {
		return fmt.Errorf("failed to git clone from %s: %s", defaultRemoteURL, err)
	}

	log.Printf("Cloned project %s to %s", project.Name, projectPath)
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

// CheckoutReference sets the current HEAD in repo to point at the revision and remote referenced by checkoutFrom
func CheckoutReference(repo *git.Repository, project *dw.Project, projectPath string) error {
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
	remote, err := repo.Remote(defaultRemoteName)
	if err != nil {
		return fmt.Errorf("could not find remote %s: %s", defaultRemoteName, err)
	}

	refs, err := remote.List(&git.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to read remote %s: %s", defaultRemoteName, err)
	}

	branch, err := repo.Branch(checkoutFrom.Revision)
	if err == nil {
		return checkoutLocalBranch(projectPath, branch.Name, defaultRemoteName)
	}

	for _, ref := range refs {
		if ref.Name().Short() != checkoutFrom.Revision {
			continue
		}
		if ref.Name().IsBranch() {
			return checkoutRemoteBranch(projectPath, defaultRemoteName, ref)
		} else if ref.Name().IsTag() {
			return checkoutTag(projectPath, defaultRemoteName, ref)
		}
	}

	log.Printf("No tag or branch named %s found on remote %s; attempting to resolve commit", checkoutFrom.Revision, defaultRemoteName)
	if _, err := repo.ResolveRevision(plumbing.Revision(checkoutFrom.Revision)); err == nil {
		return checkoutCommit(projectPath, checkoutFrom.Revision)
	}
	log.Printf("Could not find revision %s in repository, using default branch", checkoutFrom.Revision)
	return nil
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

func checkoutRemoteBranch(projectPath string, remote string, branchRef *plumbing.Reference) error {
	// Implement logic of `git checkout <remote-branch-name>`:
	// 1. Create tracking info in .git/config to properly track remote branch
	// 2. Create local branch to match name of remote branch with hash matching remote branch
	// More info: see https://git-scm.com/docs/git-checkout section `git checkout [<branch>]`
	branchName := branchRef.Name().Short()
	log.Printf("Creating branch %s to track remote branch %s from %s", branchName, branchName, remote)

	if err := shell.GitCheckoutBranch(projectPath, branchName, remote); err != nil {
		return fmt.Errorf("failed to checkout branch %s: %s", branchName, err)
	}

	return nil
}

func checkoutTag(projectPath, remote string, tagRef *plumbing.Reference) error {
	tagName := tagRef.Name().Short()
	log.Printf("Checking out tag %s from remote %s", tagName, remote)

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
