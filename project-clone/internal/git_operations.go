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

package internal

import (
	"fmt"
	"log"
	"os"
	"path"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/go-git/go-git/v5"
	gitConfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

// CloneProject clones the project specified to $PROJECTS_ROOT. Note: projects.Github is ignored as it will likely
// be removed soon.
func CloneProject(project *dw.Project) (*git.Repository, error) {
	clonePath := GetClonePath(project)
	log.Printf("Cloning project %s to %s", project.Name, clonePath)

	if len(project.Git.Remotes) == 0 {
		return nil, fmt.Errorf("project does not define remotes")
	}

	var defaultRemoteName, defaultRemoteURL string

	if project.Git.CheckoutFrom != nil {
		defaultRemoteName = project.Git.CheckoutFrom.Remote
		remoteURL, ok := project.Git.Remotes[defaultRemoteName]
		if !ok {
			return nil, fmt.Errorf("project checkoutFrom refers to non-existing remote %s", defaultRemoteName)
		}
		defaultRemoteURL = remoteURL
	} else {
		if len(project.Git.Remotes) > 1 {
			return nil, fmt.Errorf("project checkoutFrom field is required when a project defines multiple remotes")
		}
		for remoteName, remoteUrl := range project.Git.Remotes {
			defaultRemoteName, defaultRemoteURL = remoteName, remoteUrl
		}
	}

	repo, err := git.PlainClone(path.Join(projectsRoot, clonePath), false, &git.CloneOptions{
		URL:        defaultRemoteURL,
		RemoteName: defaultRemoteName,
		Progress:   os.Stdout,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to git clone from %s: %s", defaultRemoteURL, err)
	}

	log.Printf("Cloned project %s to %s", project.Name, clonePath)
	return repo, nil
}

// SetupRemotes sets up a git remote in repo for each remote in project.Git.Remotes
func SetupRemotes(repo *git.Repository, project *dw.Project) error {
	log.Printf("Setting up remotes for project %s", project.Name)
	for remoteName, remoteUrl := range project.Git.Remotes {
		_, err := repo.CreateRemote(&gitConfig.RemoteConfig{
			Name: remoteName,
			URLs: []string{remoteUrl},
		})
		if err != nil && err != git.ErrRemoteExists {
			return fmt.Errorf("failed to add remote %s: %s", remoteName, err)
		}
		err = repo.Fetch(&git.FetchOptions{
			RemoteName: remoteName,
		})
		if err != nil && err != git.NoErrAlreadyUpToDate {
			return fmt.Errorf("failed to fetch from remote %s: %s", remoteUrl, err)
		}
		log.Printf("Fetched remote %s at %s", remoteName, remoteUrl)
	}
	return nil
}

// CheckoutReference sets the current HEAD in repo to point at the revision and remote referenced by checkoutFrom
func CheckoutReference(repo *git.Repository, checkoutFrom *dw.CheckoutFrom) error {
	if checkoutFrom == nil {
		return nil
	}
	remote, err := repo.Remote(checkoutFrom.Remote)
	if err != nil {
		return fmt.Errorf("could not find remote %s: %s", checkoutFrom.Remote, err)
	}

	refs, err := remote.List(&git.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to read remote %s: %s", checkoutFrom.Remote, err)
	}

	for _, ref := range refs {
		if ref.Name().Short() != checkoutFrom.Revision {
			continue
		}
		if ref.Name().IsBranch() {
			return checkoutRemoteBranch(repo, checkoutFrom.Remote, ref)
		} else if ref.Name().IsTag() {
			return checkoutTag(repo, checkoutFrom.Remote, ref)
		}
	}

	log.Printf("No tag or branch named %s found on remote %s; attempting to resolve commit", checkoutFrom.Revision, checkoutFrom.Remote)
	hash, err := repo.ResolveRevision(plumbing.Revision(checkoutFrom.Revision))
	if err != nil {
		return fmt.Errorf("failed to resolve commit %s: %s", checkoutFrom.Revision, err)
	}
	return checkoutCommit(repo, hash)
}

func checkoutRemoteBranch(repo *git.Repository, remote string, branchRef *plumbing.Reference) error {
	// Implement logic of `git checkout <remote-branch-name>`:
	// 1. Create tracking info in .git/config to properly track remote branch
	// 2. Create local branch to match name of remote branch with hash matching remote branch
	// More info: see https://git-scm.com/docs/git-checkout section `git checkout [<branch>]`
	branchName := branchRef.Name().Short()
	log.Printf("Creating branch %s to track remote branch %s from %s", branchName, branchName, remote)

	// repo.CreateBranch does _not_ do the equivalent of `git branch <branch-name>`. It only creates the tracking
	// config in `.git/config` but leaves the current repos refs alone.
	err := repo.CreateBranch(&gitConfig.Branch{
		Name:   branchName,
		Remote: remote,
		Merge:  branchRef.Name(),
	})
	if err != nil {
		return fmt.Errorf("failed to create local branch %s: %s", branchName, err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get current worktree: %s", err)
	}
	err = worktree.Checkout(&git.CheckoutOptions{
		Hash:   branchRef.Hash(),
		Branch: plumbing.NewBranchReferenceName(branchName),
		Create: true,
	})
	if err != nil {
		return fmt.Errorf("failed to checkout branch %s: %s", branchName, err)
	}

	return nil
}

func checkoutTag(repo *git.Repository, remote string, tagRef *plumbing.Reference) error {
	tagName := tagRef.Name().Short()
	log.Printf("Checking out tag %s from remote %s", tagName, remote)

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get current worktree: %s", err)
	}
	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewTagReferenceName(tagName),
	})
	if err != nil {
		return fmt.Errorf("failed to checkout tag %s: %s", tagName, err)
	}

	return nil
}

func checkoutCommit(repo *git.Repository, hash *plumbing.Hash) error {
	log.Printf("Checking out commit %s", hash)

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get current worktree: %s", err)
	}
	err = worktree.Checkout(&git.CheckoutOptions{
		Hash: *hash,
	})
	if err != nil {
		return fmt.Errorf("failed to checkout commit %s: %s", hash, err)
	}
	return nil
}
