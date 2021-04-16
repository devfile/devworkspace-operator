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

// Package projects defines library functions for reconciling projects in a Devfile (i.e. cloning and maintaining state)
package projects

import (
	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"

	"github.com/devfile/devworkspace-operator/internal/images"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

const (
	projectClonerContainerName = "project-clone"
	projectClonerCommandID     = "clone-projects"
)

func AddProjectClonerComponent(workspace *dw.DevWorkspaceTemplateSpec) {
	if len(workspace.Projects) == 0 {
		return
	}
	cloneImage := images.GetProjectClonerImage()
	if cloneImage == "" {
		return
	}
	container := getProjectClonerContainer(cloneImage)
	command := getProjectClonerCommand()
	workspace.Components = append(workspace.Components, *container)
	workspace.Commands = append(workspace.Commands, *command)
	if workspace.Events == nil {
		workspace.Events = &dw.Events{}
	}
	workspace.Events.PreStart = append(workspace.Events.PreStart, projectClonerCommandID)
}

func getProjectClonerContainer(projectCloneImage string) *dw.Component {
	boolTrue := true
	return &dw.Component{
		Name: projectClonerContainerName,
		ComponentUnion: dw.ComponentUnion{
			Container: &dw.ContainerComponent{
				Container: dw.Container{
					Image:         projectCloneImage,
					MemoryLimit:   constants.ProjectCloneMemoryLimit,
					MemoryRequest: constants.ProjectCloneMemoryRequest,
					CpuLimit:      constants.ProjectCloneCPULimit,
					CpuRequest:    constants.ProjectCloneCPURequest,
					MountSources:  &boolTrue,
				},
			},
		},
	}
}

func getProjectClonerCommand() *dw.Command {
	return &dw.Command{
		Id: projectClonerCommandID,
		CommandUnion: dw.CommandUnion{
			Apply: &dw.ApplyCommand{
				Component: projectClonerContainerName,
			},
		},
	}
}
