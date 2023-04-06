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

package lifecycle

import (
	"fmt"
	"strings"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	corev1 "k8s.io/api/core/v1"
)

func AddPostStartLifecycleHooks(wksp *dw.DevWorkspaceTemplateSpec, containers []corev1.Container) error {
	if wksp.Events == nil || len(wksp.Events.PostStart) == 0 {
		return nil
	}

	componentToCommands := map[string][]dw.Command{}
	for _, commandName := range wksp.Events.PostStart {
		command, err := getCommandByKey(commandName, wksp.Commands)
		if err != nil {
			return fmt.Errorf("could not resolve command for postStart event '%s': %w", commandName, err)
		}
		cmdType, err := getCommandType(*command)
		if err != nil {
			return fmt.Errorf("could not determine command type for '%s': %w", command.Key(), err)
		}
		if cmdType != dw.ExecCommandType {
			return fmt.Errorf("can not use %s-type command in postStart lifecycle event", cmdType)
		}

		componentToCommands[command.Exec.Component] = append(componentToCommands[command.Exec.Component], *command)
	}

	for componentName, commands := range componentToCommands {
		cmdContainer, err := getContainerWithName(componentName, containers)
		if err != nil {
			return fmt.Errorf("failed to process postStart event %s: %w", commands[0].Id, err)
		}

		postStartHandler, err := processCommandsForPostStart(commands)
		if err != nil {
			return fmt.Errorf("failed to process postStart event %s: %w", commands[0].Id, err)
		}

		if cmdContainer.Lifecycle == nil {
			cmdContainer.Lifecycle = &corev1.Lifecycle{}
		}
		cmdContainer.Lifecycle.PostStart = postStartHandler
	}

	return nil
}

// processCommandsForPostStart builds a lifecycle handler that runs the provided command(s)
// The command has the format
//
// exec:
//
//	command:
//	  - "/bin/sh"
//	  - "-c"
//	  - |
//	    cd <workingDir>
//	    <commandline>
func processCommandsForPostStart(commands []dw.Command) (*corev1.LifecycleHandler, error) {
	var dwCommands []string
	for _, command := range commands {
		execCmd := command.Exec
		if len(execCmd.Env) > 0 {
			return nil, fmt.Errorf("env vars in postStart command %s are unsupported", command.Id)
		}
		if execCmd.WorkingDir != "" {
			dwCommands = append(dwCommands, fmt.Sprintf("cd %s", execCmd.WorkingDir))
		}
		dwCommands = append(dwCommands, execCmd.CommandLine)
	}

	handler := &corev1.LifecycleHandler{
		Exec: &corev1.ExecAction{
			Command: []string{
				"/bin/sh",
				"-c",
				strings.Join(dwCommands, "\n"),
			},
		},
	}
	return handler, nil
}

func getContainerWithName(name string, containers []corev1.Container) (*corev1.Container, error) {
	for idx, container := range containers {
		if container.Name == name {
			return &containers[idx], nil
		}
	}
	return nil, fmt.Errorf("container component with name %s not found", name)
}
