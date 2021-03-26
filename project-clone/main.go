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
	"path"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/go-git/go-git/v5"
	gitConfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"sigs.k8s.io/yaml"

	"github.com/devfile/devworkspace-operator/pkg/library/constants"
	"github.com/devfile/devworkspace-operator/pkg/provision/metadata"
)

var (
	projectsRoot              string
	flattenedDevWorkspacePath string
)

func init() {
	projectsRoot = os.Getenv(constants.ProjectsRootEnvVar)
	if projectsRoot == "" {
		log.Printf("Required environment variable %s is unset", constants.ProjectsRootEnvVar)
		os.Exit(1)
	}

	flattenedDevWorkspacePath = os.Getenv(metadata.FlattenedDevfileMountPathEnvVar)
	if flattenedDevWorkspacePath == "" {
		log.Printf("Required environment variable %s is unset", metadata.FlattenedDevfileMountPathEnvVar)
		os.Exit(1)
	}
}

func main() {
	workspace, err := readFlattenedDevWorkspace()
	if err != nil {
		log.Printf("Failed to read current DevWorkspace: %s", err)
		os.Exit(1)
	}
	for _, project := range workspace.Projects {
		needClone, needRemotes, err := checkProjectState(&project)
		if err != nil {
			log.Printf("Failed to process project %s: %s", project.Name, err)
			os.Exit(1)
		}
		if needClone {
			repo, err := cloneProject(&project)
			if err != nil {
				log.Printf("failed to clone project %s: %s", project.Name, err)
				os.Exit(1)
			}
			if err := setupRemotes(repo, &project); err != nil {
				log.Printf("Failed to set up remotes for project %s: %s", project.Name, err)
				os.Exit(1)
			}
			if err := handleCheckoutFromReference(repo, &project); err != nil {
				log.Printf("Failed to checkout revision for project %s: %s", project.Name, err)
			}
		} else if needRemotes {
			repo, err := openRepo(&project)
			if err != nil {
				log.Printf("Failed to open existing project %s: %s", project.Name, err)
			} else if repo == nil {
				log.Printf("Unexpected error while setting up remotes for project %s: git repository not present", project.Name)
			}
			if err := setupRemotes(repo, &project); err != nil {
				log.Printf("Failed to set up remotes for project %s: %s", project.Name, err)
				os.Exit(1)
			}
		} else {
			log.Printf("Project '%s' is already cloned and has all remotes configured", project.Name)
		}
	}
}

func checkProjectState(project *dw.Project) (needClone, needRemotes bool, err error) {
	repo, err := openRepo(project)
	if err != nil {
		return false, false, err
	}
	if repo == nil {
		return true, true, nil
	}

	for projRemote, projRemoteURL := range project.Git.Remotes {
		gitRemote, err := repo.Remote(projRemote)
		if err != nil {
			if err == git.ErrRemoteNotFound {
				return false, true, nil
			}
			return false, false, fmt.Errorf("error reading remotes: %s", err)
		}
		found := false
		for _, remoteUrl := range gitRemote.Config().URLs {
			if remoteUrl == projRemoteURL {
				found = true
			}
		}
		if !found {
			return false, true, nil
		}
	}
	return false, false, nil
}

func setupRemotes(repo *git.Repository, project *dw.Project) error {
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

// cloneProject clones the project specified to $PROJECTS_ROOT. Note: projects.Github is ignored as it will likely
// be removed soon.
// TODO: Handle projects specifying a zip file instead of git
// TODO: Handle sparse checkout
// TODO: Add support for auth
func cloneProject(project *dw.Project) (*git.Repository, error) {
	clonePath := getClonePath(project)
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

func handleCheckoutFromReference(repo *git.Repository, project *dw.Project) error {
	if project.Git == nil || project.Git.CheckoutFrom == nil || project.Git.CheckoutFrom.Revision == "" {
		return nil
	}
	defaultReference := project.Git.CheckoutFrom.Revision
	if defaultReference != "" {
		checkoutOptions, err := resolveDefaultCheckout(repo, defaultReference)
		if err != nil {
			return fmt.Errorf("error resolving reference %s: %s", defaultReference, err)
		}
		log.Printf("Checking out %s in project %s", defaultReference, project.Name)
		w, err := repo.Worktree()
		if err != nil {
			return fmt.Errorf("failed to read worktree: %s", err)
		}
		if err := w.Checkout(checkoutOptions); err != nil {
			return fmt.Errorf("error checking out reference %s: %s", defaultReference, err)
		}
	}
	return nil
}

func resolveDefaultCheckout(repo *git.Repository, ref string) (*git.CheckoutOptions, error) {
	branch, err := repo.Branch(ref)
	if err == nil {
		log.Printf("Resolved branch %s", branch.Name)
		return &git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName(branch.Name),
		}, nil
	}
	if err != git.ErrBranchNotFound {
		return nil, err
	}
	tag, err := repo.Tag(ref)
	if err == nil {
		log.Printf("Resolved tag %s", tag.Name())
		return &git.CheckoutOptions{
			Branch: plumbing.NewTagReferenceName(tag.String()),
		}, nil
	}
	if err != git.ErrTagNotFound {
		return nil, err
	}
	commit, err := repo.CommitObject(plumbing.NewHash(ref))
	if err == nil {
		log.Printf("Resolved commit %s", commit.Hash)
		return &git.CheckoutOptions{
			Hash: commit.Hash,
		}, nil
	}
	if err != plumbing.ErrObjectNotFound {
		return nil, err
	}
	return nil, fmt.Errorf("could not resolve checkoutFrom reference")
}

func getClonePath(project *dw.Project) string {
	if project.ClonePath != "" {
		return project.ClonePath
	}
	return project.Name
}

// openRepo returns the git repo on disk described by the devworkspace Project. If the repo does not
// currently exist, returns nil. Returns an error if an unexpected error occurs opening the git repo.
func openRepo(project *dw.Project) (*git.Repository, error) {
	clonePath := getClonePath(project)
	repo, err := git.PlainOpen(path.Join(projectsRoot, clonePath))
	if err != nil {
		if err != git.ErrRepositoryNotExists {
			return nil, fmt.Errorf("encountered error reading git repo at %s: %s", clonePath, err)
		}
		return nil, nil
	}
	return repo, nil
}

func readFlattenedDevWorkspace() (*dw.DevWorkspaceTemplateSpec, error) {
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
