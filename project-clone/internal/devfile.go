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
	"io/ioutil"
	"log"
	"os"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"sigs.k8s.io/yaml"

	"github.com/devfile/devworkspace-operator/pkg/provision/metadata"
)

// GetClonePath gets the correct clonePath for a project, given the semantics in devfile/api
func GetClonePath(project *dw.Project) string {
	if project.ClonePath != "" {
		return project.ClonePath
	}
	return project.Name
}

// ReadFlattenedDevWorkspace reads the flattened DevWorkspaceTemplateSpec from disk. The location of the flattened
// yaml is determined from the DevWorkspace Operator-provisioned environment variable.
func ReadFlattenedDevWorkspace() (*dw.DevWorkspaceTemplateSpec, error) {
	flattenedDevWorkspacePath := os.Getenv(metadata.FlattenedDevfileMountPathEnvVar)
	if flattenedDevWorkspacePath == "" {
		return nil, fmt.Errorf("required environment variable %s is unset", metadata.FlattenedDevfileMountPathEnvVar)
	}

	fileBytes, err := ioutil.ReadFile(flattenedDevWorkspacePath)
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
