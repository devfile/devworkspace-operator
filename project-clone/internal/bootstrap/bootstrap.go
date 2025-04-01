// Copyright (c) 2019-2025 Red Hat, Inc.
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
	"context"
	"fmt"
	"log"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

var devfileNames = []string{"devfile.yaml", ".devfile.yaml"}

func NeedsBootstrap(dw *dw.DevWorkspaceTemplateSpec) (bool, error) {
	if dw.Attributes == nil {
		return false, nil
	}
	if !dw.Attributes.Exists(constants.BootstrapDevWorkspaceAttribute) {
		return false, nil
	}
	var attrErr error
	needBootstrap := dw.Attributes.GetBoolean(constants.BootstrapDevWorkspaceAttribute, &attrErr)
	if attrErr != nil {
		return false, attrErr
	}
	return needBootstrap, nil
}

func BootstrapWorkspace(flattenedWorkspace *dw.DevWorkspaceTemplateSpec) error {
	devfile, projectName, err := getBootstrapDevfile(flattenedWorkspace.Projects)
	if err != nil {
		return err
	}
	log.Printf("Updating current DevWorkspace with content from devfile found in project %s", projectName)

	kubeclient, err := setupKubeClient()
	if err != nil {
		return err
	}

	workspaceNN, err := getWorkspaceNamespacedName()
	if err != nil {
		return err
	}

	clusterWorkspace := &dw.DevWorkspace{}
	if err := kubeclient.Get(context.Background(), workspaceNN, clusterWorkspace); err != nil {
		return fmt.Errorf("failed to read DevWorkspace from cluster: %w", err)
	}

	updatedWorkspace := updateWorkspaceFromDevfile(clusterWorkspace, devfile)

	if err := kubeclient.Update(context.Background(), updatedWorkspace); err != nil {
		return fmt.Errorf("failed to update DevWorkspace on cluster: %w", err)
	}

	log.Printf("Successfully updated DevWorkspace. Workspace may restart")

	return nil
}

func updateWorkspaceFromDevfile(workspace *dw.DevWorkspace, devfile *dw.DevWorkspaceTemplateSpec) *dw.DevWorkspace {
	updated := workspace.DeepCopy()

	// Use devfile contents for this DevWorkspace instead of whatever is there
	updated.Spec.Template = *devfile

	// Add attributes from original DevWorkspace, since it's assumed they're more relevant than the devfile's attributes
	// This will override any attributes present in both the devfile and DevWorkspace with the latter's value
	if updated.Spec.Template.Attributes == nil {
		updated.Spec.Template.Attributes = attributes.Attributes{}
	}
	for key, value := range workspace.Spec.Template.Attributes {
		updated.Spec.Template.Attributes[key] = value
	}

	// Merge projects; we want the DevWorkspace's projects to not be dropped from the workspace, but also want to add any projects
	// present in the devfile. We also want workspace projects first in this list, since this is the order they're bootstrapped from
	updated.Spec.Template.Projects = mergeProjects(workspace.Spec.Template.Projects, devfile.Projects)

	// Remove bootstrap attribute to avoid unnecessarily doing this process in the future
	delete(updated.Spec.Template.Attributes, constants.BootstrapDevWorkspaceAttribute)

	return updated
}

func mergeProjects(workspaceProjects, devfileProjects []dw.Project) []dw.Project {
	var allProjects []dw.Project

	// Bookkeeping structs to avoid adding identical projects. We want to avoid an issue where DevWorkspace and devfile
	// contain the same project; adding both to the workspace will cause the workspace to be invalid.
	// An additional improvement in the future would be to avoid adding two very similar projects (e.g. identical projects
	// with different names)
	projectNames := map[string]bool{}

	for _, project := range workspaceProjects {
		projectNames[project.Name] = true
		allProjects = append(allProjects, project)
	}

	for _, project := range devfileProjects {
		if !projectNames[project.Name] {
			projectNames[project.Name] = true
			allProjects = append(allProjects, project)
		}
	}

	return allProjects
}
