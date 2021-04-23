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
	"path"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/go-git/go-git/v5"
)

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
	repo, err := git.PlainOpen(path.Join(projectsRoot, clonePath))
	if err != nil {
		if err != git.ErrRepositoryNotExists {
			return nil, fmt.Errorf("encountered error reading git repo at %s: %s", clonePath, err)
		}
		return nil, nil
	}
	return repo, nil
}
