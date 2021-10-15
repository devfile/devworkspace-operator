//
// Copyright (c) 2019-2021 Red Hat, Inc.
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
)

// Read and store ProjectsRoot env var for reuse throughout project-clone.
func init() {
	ProjectsRoot = os.Getenv(constants.ProjectsRootEnvVar)
	if ProjectsRoot == "" {
		log.Printf("Required environment variable %s is unset", constants.ProjectsRootEnvVar)
		os.Exit(1)
	}
}
