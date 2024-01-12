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
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/devfile/devworkspace-operator/pkg/library/constants"
	gittransport "github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/kevinburke/ssh_config"
)

const (
	credentialsMountPath = "/.git-credentials/credentials"
	sshConfigMountPath   = "/etc/ssh/ssh_config"
	publicCertsDir       = "/public-certs"
)

var (
	ProjectsRoot     string
	CloneTmpDir      string
	tokenAuthMethod  map[string]*githttp.BasicAuth
	credentialsRegex = regexp.MustCompile(`https://(.+):(.+)@(.+)`)
)

// Read and store ProjectsRoot env var for reuse throughout project-clone.
func init() {
	ProjectsRoot = os.Getenv(constants.ProjectsRootEnvVar)
	if ProjectsRoot == "" {
		log.Printf("Required environment variable %s is unset", constants.ProjectsRootEnvVar)
		os.Exit(1)
	}
	// Have to use path within PROJECTS_ROOT in case it is a mounted directory; otherwise, moving files will fail
	// (os.Rename fails when source and dest are on different partitions)
	tmpDir, err := os.MkdirTemp(ProjectsRoot, "project-clone-")
	if err != nil {
		log.Printf("Failed to get temporary directory for setting up projects: %s", err)
		os.Exit(1)
	}
	log.Printf("Using temporary directory %s", tmpDir)
	CloneTmpDir = tmpDir

	setupAuth()
}

func GetAuthForHost(repoURLStr string) (gittransport.AuthMethod, error) {
	endpoint, err := gittransport.NewEndpoint(repoURLStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %w", err)
	}
	switch endpoint.Protocol {
	case "ssh":
		identityFiles := ssh_config.GetAll(endpoint.Host, "IdentityFile")
		if len(identityFiles) == 0 {
			log.Printf("No SSH key found for host %s", endpoint.Host)
		} else if len(identityFiles) > 1 {
			// Probably should try all keys, one by one, in the future
			log.Printf("Warning: multiple SSH keys found for host %s. Using first match.", endpoint.Host)
		}
		user := ssh_config.Get(endpoint.Host, "User")
		if user == "" {
			user = "git"
		}
		pubkeys, err := gitssh.NewPublicKeysFromFile(user, identityFiles[0], "")
		if err != nil {
			return nil, fmt.Errorf("failed to set up SSH: %w", err)
		}
		return pubkeys, nil
	case "http", "https":
		authMethod, ok := tokenAuthMethod[endpoint.Host]
		if !ok {
			log.Printf("No personal access token found for URL %s", repoURLStr)
			return nil, nil
		}
		log.Printf("Found personal access token for URL %s", repoURLStr)
		return authMethod, nil
	default:
		log.Printf("No personal access token for URL %s; unsupported protocol: %s", repoURLStr, endpoint.Protocol)
	}
	return nil, nil
}

func setupAuth() {
	gitCredentials, err := os.ReadFile(credentialsMountPath)
	if err != nil {
		// If file does not exist, no credentials to mount
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("Unexpected error reading git credentials file: %s", err)
			os.Exit(1)
		}
	} else {
		tokenAuthMethod = parseCredentialsFile(string(gitCredentials))
	}
}

func parseCredentialsFile(gitCredentials string) map[string]*githttp.BasicAuth {
	result := map[string]*githttp.BasicAuth{}
	matches := credentialsRegex.FindAllStringSubmatch(gitCredentials, -1)
	for idx, credential := range matches {
		if len(credential) != 4 {
			log.Printf("Malformed credential found in credentials file on line %d. Skipping", idx+1)
		}
		username := credential[1]
		pat := credential[2]
		url := credential[3]
		result[url] = &githttp.BasicAuth{
			Username: username,
			Password: pat,
		}
	}
	return result
}
