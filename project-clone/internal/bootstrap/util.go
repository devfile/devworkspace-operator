// Copyright (c) 2019-2023 Red Hat, Inc.
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

package bootstrap

import (
	"fmt"
	"os"
	"path"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/library/projects"
	"github.com/devfile/devworkspace-operator/project-clone/internal"
	"sigs.k8s.io/yaml"
)

func getBootstrapDevfile(projects []dw.Project) (devfile *dw.DevWorkspaceTemplateSpec, projectName string, err error) {
	var devfileBytes []byte
	var devfileProject string
	for _, project := range projects {
		bytes, err := getDevfileFromProject(project)
		if err == nil && len(bytes) > 0 {
			devfileBytes = bytes
			devfileProject = project.Name
			break
		}
	}
	if len(devfileBytes) == 0 {
		return nil, "", fmt.Errorf("could not find devfile in any project")
	}

	devfile = &dw.DevWorkspaceTemplateSpec{}
	if err := yaml.Unmarshal(devfileBytes, devfile); err != nil {
		return nil, "", fmt.Errorf("failed to read devfile in project %s: %s", devfileProject, err)
	}
	return devfile, devfileProject, nil
}

func getDevfileFromProject(project dw.Project) ([]byte, error) {
	clonePath := projects.GetClonePath(&project)
	for _, devfileName := range devfileNames {
		if bytes, err := os.ReadFile(path.Join(internal.ProjectsRoot, clonePath, devfileName)); err == nil {
			return bytes, nil
		}
	}
	return nil, fmt.Errorf("no devfile found")
}
