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

package internal

import (
	"crypto/x509"
	"fmt"
	"os"
	"path"
	"path/filepath"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	projectslib "github.com/devfile/devworkspace-operator/pkg/library/projects"
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
	if project.Attributes.Exists(ProjectSubDir) {
		return checkSubPathProjectState(project)
	}
	repo, err := OpenRepo(path.Join(ProjectsRoot, projectslib.GetClonePath(project)))
	if err != nil {
		return false, false, err
	}
	if repo == nil {
		return true, false, nil
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
func OpenRepo(repoPath string) (*git.Repository, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		if err != git.ErrRepositoryNotExists {
			return nil, fmt.Errorf("encountered error reading git repo at %s: %s", repoPath, err)
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

func checkSubPathProjectState(project *dw.Project) (needClone, needRemotes bool, err error) {
	// Check that attribute is valid and parseable
	var attrErr error
	_ = project.Attributes.GetString(ProjectSubDir, &attrErr)
	if err != nil {
		return false, false, fmt.Errorf("failed to read %s attribute on project %s: %w", ProjectSubDir, project.Name, err)
	}

	// Result here won't be a git repository, so we only check whether the directory exists
	clonePath := path.Join(ProjectsRoot, projectslib.GetClonePath(project))
	stat, err := os.Stat(clonePath)
	if err != nil {
		if os.IsNotExist(err) {
			return true, false, nil
		}
		return false, false, err
	}
	if stat.IsDir() {
		return false, false, nil
	} else {
		return false, false, fmt.Errorf("could not check project state for project %s -- path %s exists and is not a directory", project.Name, clonePath)
	}
}

func GetAdditionalCerts() (certs *x509.CertPool, warnings []string, err error) {
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		return nil, warnings, fmt.Errorf("could not read system certificates pool: %w", err)
	}

	certFiles, err := os.ReadDir(publicCertsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return rootCAs, warnings, nil
		}
		return nil, warnings, fmt.Errorf("could not read files in %s: %w", publicCertsDir, err)
	}
	for _, certFile := range certFiles {
		if certFile.IsDir() {
			continue
		}
		ext := filepath.Ext(certFile.Name())
		if ext == ".crt" || ext == ".pem" {
			fullPath := path.Join(publicCertsDir, certFile.Name())
			fileBytes, err := os.ReadFile(fullPath)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("Failed to read certificate %s: %s", fullPath, err))
			}
			if ok := rootCAs.AppendCertsFromPEM(fileBytes); !ok {
				warnings = append(warnings, fmt.Sprintf("Failed to append certificate %s to pool", fullPath))
			}
		}
	}
	return rootCAs, warnings, nil
}
