//
// Copyright (c) 2019-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package restapis

import (
	"errors"
	"path"
	"strings"

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha2"
	workspaceApi "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config"
)

func devworkspaceTemplateToDevfileV1(template *devworkspace.DevWorkspaceTemplateSpec) (*workspaceApi.DevfileSpec, error) {
	devfile := &workspaceApi.DevfileSpec{}
	for _, templateCommand := range template.Commands {
		command := toDevfileCommand(templateCommand)
		if command != nil {
			devfile.Commands = append(devfile.Commands, *command)
		}
	}

	for _, templateComponent := range template.Components {
		component := toDevfileComponent(templateComponent)
		if component != nil {
			devfile.Components = append(devfile.Components, *component)
		}
	}

	for _, templateProject := range template.Projects {
		project, err := toDevfileProject(templateProject)
		if err != nil {
			return nil, err
		}
		if project != nil {
			devfile.Projects = append(devfile.Projects, *project)
		}
	}
	return devfile, nil
}

func toDevfileCommand(cmd devworkspace.Command) *workspaceApi.CommandSpec {
	var commandSpec *workspaceApi.CommandSpec = nil
	switch {
	case cmd.Exec != nil:
		name := cmd.Exec.Label
		if name == "" {
			name = cmd.Id
		}
		commandSpec = &workspaceApi.CommandSpec{
			Name: name,
			Actions: []workspaceApi.CommandActionSpec{
				{
					Command:   cmd.Exec.CommandLine,
					Component: cmd.Exec.Component,
					Workdir:   cmd.Exec.WorkingDir,
					Type:      "exec",
				},
			},
		}
	case cmd.VscodeLaunch != nil:
		commandSpec = &workspaceApi.CommandSpec{
			Name: cmd.Id,
			Actions: []workspaceApi.CommandActionSpec{
				{
					Type:             "vscode-launch",
					ReferenceContent: cmd.VscodeLaunch.Inlined,
				},
			},
		}
	case cmd.VscodeTask != nil:
		commandSpec = &workspaceApi.CommandSpec{
			Name: cmd.Id,
			Actions: []workspaceApi.CommandActionSpec{
				{
					Type:             "vscode-task",
					ReferenceContent: cmd.VscodeTask.Inlined,
				},
			},
		}
	}

	return commandSpec
}

func toDevfileEndpoints(eps []devworkspace.Endpoint) []workspaceApi.Endpoint {
	devfileEndpoints := []workspaceApi.Endpoint{}
	for _, e := range eps {
		attributes := map[workspaceApi.EndpointAttribute]string{}
		if e.Protocol != "" {
			attributes[workspaceApi.PROTOCOL_ENDPOINT_ATTRIBUTE] = string(e.Protocol)
		} else {
			attributes[workspaceApi.PROTOCOL_ENDPOINT_ATTRIBUTE] = "http"
		}

		devfileEndpoints = append(devfileEndpoints, workspaceApi.Endpoint{
			Name:       e.Name,
			Port:       int64(e.TargetPort),
			Attributes: attributes,
		})
	}
	return devfileEndpoints
}

func toDevfileComponent(c devworkspace.Component) *workspaceApi.ComponentSpec {
	var componentSpec *workspaceApi.ComponentSpec = nil
	switch {
	case c.Plugin != nil:
		if strings.Contains(c.Plugin.Id, config.TheiaEditorID) {
			componentSpec = &workspaceApi.ComponentSpec{
				Type:  workspaceApi.CheEditor,
				Alias: c.Name,
				Id:    c.Plugin.Id,
			}
		} else {
			componentSpec = &workspaceApi.ComponentSpec{
				Type:  workspaceApi.ChePlugin,
				Alias: c.Name,
				Id:    c.Plugin.Id,
			}
		}
	case c.Container != nil:
		componentSpec = &workspaceApi.ComponentSpec{
			Type:         workspaceApi.Dockerimage,
			Alias:        c.Name,
			Image:        c.Container.Image,
			MemoryLimit:  c.Container.MemoryLimit,
			MountSources: c.Container.MountSources == nil || *c.Container.MountSources,
			Endpoints:    toDevfileEndpoints(c.Container.Endpoints),
		}
	}

	return componentSpec
}

func toDevfileProject(p devworkspace.Project) (*workspaceApi.ProjectSpec, error) {
	var theLocation string
	var theType string

	err := p.Visit(devworkspace.ProjectSourceVisitor{
		Git: func(src *devworkspace.GitProjectSource) error {
			l, err := resolveLocation(src.Remotes, src.CheckoutFrom)
			if err != nil {
				return err
			}
			theLocation = *l
			theType = "git"
			return nil
		},
		Github: func(src *devworkspace.GithubProjectSource) error {
			l, err := resolveLocation(src.Remotes, src.CheckoutFrom)
			if err != nil {
				return err
			}
			theLocation = *l
			theType = "github"
			return nil
		},
		Zip: func(src *devworkspace.ZipProjectSource) error {
			theLocation = src.Location
			theType = "zip"
			return nil
		},
	})
	if err != nil {
		return nil, err
	}
	clonePath := resolveClonePath(p.ClonePath, p.Name, config.DefaultProjectsSourcesRoot)
	return &workspaceApi.ProjectSpec{
		Name: p.Name,
		Source: workspaceApi.ProjectSourceSpec{
			Location: theLocation,
			Type:     theType,
		},
		ClonePath: clonePath,
	}, nil
}

func resolveLocation(remotes map[string]string, checkoutFrom *devworkspace.CheckoutFrom) (location *string, err error) {
	if len(remotes) == 0 {
		return nil, errors.New("at least one remote is required")
	}

	if len(remotes) == 1 {
		//return the location of the only remote
		for _, l := range remotes {
			return &l, nil
		}
	}

	remote := checkoutFrom.Remote
	if remote == "" {
		return nil, errors.New("multiple remotes are specified but checkoutFrom is not configured")
	}

	l, exist := remotes[remote]
	if !exist {
		return nil, errors.New("the configured remote is not found in the remotes")
	}

	return &l, nil
}

// Clone Path is the path relative to the root of the projects to which this project should be cloned into.
// This is a unix-style relative path (i.e. uses forward slashes).
// The path is invalid if it is absolute or tries to escape the project root through the usage of '..'.
// If not specified, defaults to the project name.
func resolveClonePath(clonePath string, projectName string, projectRoot string) string {

	// clonePath isn't specified
	if clonePath == "" {
		return projectName
	}

	// Absolute paths are invalid
	isAbs := path.IsAbs(clonePath)
	if isAbs {
		return projectName
	}

	// Check to make sure that the path doesn't escape the project root
	// The idea is that since everything is relative to the root, if we find a new directory
	// then add it to the currentPath. If we find ".." then remove the last item in currentPath because
	// we've gone up a section in the currentPath. If we escape the projects root then return the projectName
	trimmedProjectRoot := strings.TrimLeft(projectRoot, "/")
	newPaths := strings.Split(clonePath, "/")
	currentPath := strings.Split(trimmedProjectRoot, "/")
	for _, newPathPart := range newPaths {
		if newPathPart == ".." {
			// If you can then move up one directory
			if len(currentPath) > 1 {
				currentPath = currentPath[:len(currentPath)-1]
			} else {
				// Tried to escape the projects root so default to projectName
				return projectName
			}
		} else {
			currentPath = append(currentPath, newPathPart)
		}
	}

	return clonePath
}
