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
	"fmt"
	"log"
	"os"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	"sigs.k8s.io/yaml"

	"github.com/devfile/devworkspace-operator/pkg/provision/metadata"
)

const (
	ProjectSparseCheckout = "sparseCheckout"
	ProjectSubDir         = "subDir"
)

// ReadFlattenedDevWorkspace reads the flattened DevWorkspaceTemplateSpec from disk. The location of the flattened
// yaml is determined from the DevWorkspace Operator-provisioned environment variable.
func ReadFlattenedDevWorkspace() (*dw.DevWorkspaceTemplateSpec, error) {
	flattenedDevWorkspacePath := os.Getenv(metadata.FlattenedDevfileMountPathEnvVar)
	if flattenedDevWorkspacePath == "" {
		return nil, fmt.Errorf("required environment variable %s is unset", metadata.FlattenedDevfileMountPathEnvVar)
	}

	fileBytes, err := os.ReadFile(flattenedDevWorkspacePath)
	if err != nil {
		return nil, fmt.Errorf("error reading YAML file: %s", err)
	}

	dwts := &dw.DevWorkspaceTemplateSpec{}
	if err := yaml.Unmarshal(fileBytes, dwts); err != nil {
		return nil, fmt.Errorf("error unmarshalling DevWorkspace YAML: %s", err)
	}

	log.Printf("Read DevWorkspace at %s", flattenedDevWorkspacePath)
	return dwts, nil
}

// StarterProjectToRegularProject converts a starter project defined in a DevWorkspace to a standard Project for
// easier handling
func StarterProjectToRegularProject(starterProject *dw.StarterProject) dw.Project {
	// Note: starter project does not allow for specifying clonePath
	project := dw.Project{
		Name:          starterProject.Name,
		Attributes:    starterProject.Attributes,
		ProjectSource: starterProject.ProjectSource,
	}

	if starterProject.SubDir != "" {
		if project.Attributes == nil {
			project.Attributes = attributes.Attributes{}
		}
		project.Attributes.PutString(ProjectSubDir, starterProject.SubDir)
	}

	return project
}
