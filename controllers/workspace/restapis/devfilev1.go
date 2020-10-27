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
	"strings"

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha1"
	workspaceApi "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config"
)

func devworkspaceTemplateToDevfileV1(template *devworkspace.DevWorkspaceTemplateSpec) *workspaceApi.DevfileSpec {
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
		project := toDevfileProject(templateProject)
		if project != nil {
			devfile.Projects = append(devfile.Projects, *project)
		}
	}
	return devfile
}

func toDevfileCommand(c devworkspace.Command) *workspaceApi.CommandSpec {
	var commandSpec *workspaceApi.CommandSpec = nil
	c.Visit(devworkspace.CommandVisitor{
		Exec: func(cmd *devworkspace.ExecCommand) error {
			name := cmd.Label
			if name == "" {
				name = cmd.Id
			}
			commandSpec = &workspaceApi.CommandSpec{
				Name: name,
				Actions: []workspaceApi.CommandActionSpec{
					{
						Command:   cmd.CommandLine,
						Component: cmd.Component,
						Workdir:   cmd.WorkingDir,
						Type:      "exec",
					},
				},
			}
			return nil
		},
		VscodeLaunch: func(cmd *devworkspace.VscodeConfigurationCommand) error {
			commandSpec = &workspaceApi.CommandSpec{
				Name: cmd.Id,
				Actions: []workspaceApi.CommandActionSpec{
					{
						Type:             "vscode-launch",
						ReferenceContent: cmd.Inlined,
					},
				},
			}
			return nil
		},
		VscodeTask: func(cmd *devworkspace.VscodeConfigurationCommand) error {
			commandSpec = &workspaceApi.CommandSpec{
				Name: cmd.Id,
				Actions: []workspaceApi.CommandActionSpec{
					{
						Type:             "vscode-task",
						ReferenceContent: cmd.Inlined,
					},
				},
			}
			return nil
		},
	})

	return commandSpec
}

func toDevfileEndpoints(eps []devworkspace.Endpoint) []workspaceApi.Endpoint {
	devfileEndpoints := []workspaceApi.Endpoint{}
	for _, e := range eps {
		attributes := map[workspaceApi.EndpointAttribute]string{}
		if e.Protocol != "" {
			attributes[workspaceApi.PROTOCOL_ENDPOINT_ATTRIBUTE] = e.Protocol
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
	c.Visit(devworkspace.ComponentVisitor{
		Plugin: func(plugin *devworkspace.PluginComponent) error {
			if strings.Contains(plugin.Id, config.TheiaEditorID) {
				componentSpec = &workspaceApi.ComponentSpec{
					Type:  workspaceApi.CheEditor,
					Alias: plugin.Name,
					Id:    plugin.Id,
				}
			} else {
				componentSpec = &workspaceApi.ComponentSpec{
					Type:  workspaceApi.ChePlugin,
					Alias: plugin.Name,
					Id:    plugin.Id,
				}
			}
			return nil
		},
		Container: func(container *devworkspace.ContainerComponent) error {
			componentSpec = &workspaceApi.ComponentSpec{
				Type:         workspaceApi.Dockerimage,
				Alias:        c.Container.Name,
				Image:        c.Container.Image,
				MemoryLimit:  c.Container.MemoryLimit,
				MountSources: c.Container.MountSources,
				Endpoints:    toDevfileEndpoints(c.Container.Endpoints),
			}
			return nil
		},
		Kubernetes: func(k8s *devworkspace.KubernetesComponent) error {
			return nil
		},
		Openshift: func(os *devworkspace.OpenshiftComponent) error {
			return nil
		},
		Volume: func(vol *devworkspace.VolumeComponent) error {
			return nil
		},
	})

	return componentSpec
}

func toDevfileProject(p devworkspace.Project) *workspaceApi.ProjectSpec {
	var theLocation string
	var theType string

	p.Visit(devworkspace.ProjectSourceVisitor{
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
	return &workspaceApi.ProjectSpec{
		Name: p.Name,
		Source: workspaceApi.ProjectSourceSpec{
			Location: theLocation,
			Type:     theType,
		},
	}
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
