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
	if workspace.Attributes.GetString(constants.ProjectCloneAttribute, nil) == constants.ProjectCloneDisable {
		return
	}
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
