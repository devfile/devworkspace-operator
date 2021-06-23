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
	"os"
	"path"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/go-git/go-git/v5"
)

// CheckProjectState Checks that a project's configuration is reflected in an on-disk git repository.
// - Returns needClone == true if the project has not yet been cloned
// - Returns needRemotes == true if the remotes configured in the project are not available in the on-disk repo
//
// Remotes in provided project are checked against what is configured in the git repo, but only in one direction.
// The git repo can have additional remotes -- they will be ignored here. If both the project and git repo have remote
// A configured, but the corresponding remote URL is different, needRemotes will be true.
func CheckProjectState(project *dw.Project) (needClone, needRemotes bool, err error) {
	repo, err := OpenRepo(project)
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

// OpenRepo returns the git repo on disk described by the devworkspace Project. If the repo does not
// currently exist, returns nil. Returns an error if an unexpected error occurs opening the git repo.
func OpenRepo(project *dw.Project) (*git.Repository, error) {
	clonePath := GetClonePath(project)
	repo, err := git.PlainOpen(path.Join(ProjectsRoot, clonePath))
	if err != nil {
		if err != git.ErrRepositoryNotExists {
			return nil, fmt.Errorf("encountered error reading git repo at %s: %s", clonePath, err)
		}
		return nil, nil
	}
	return repo, nil
}

// DirExists returns true if the path at dir exists and is a directory. Returns an error if the path
// exists in the filesystem but does not refer to a directory.
func DirExists(dir string) (bool, error) {
	fileInfo, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		} else {
			return false, err
		}
	}
	if fileInfo.IsDir() {
		return true, nil
	}
	return false, fmt.Errorf("path %s already exists and is not a directory", dir)
}
