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

package internal

import (
	"log"
	"os"

	"github.com/devfile/devworkspace-operator/pkg/library/constants"
)

var (
	ProjectsRoot string
	CloneTmpDir  string
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
}
