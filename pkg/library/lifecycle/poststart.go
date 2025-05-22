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

package lifecycle

import (
	"fmt"
	"strings"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	corev1 "k8s.io/api/core/v1"
)

const (
	// `tee` both stdout and stderr to files and to the main output streams.
	redirectOutputFmt = `{
  # This script block ensures its exit code is preserved
  # while its stdout and stderr are tee'd.
  _script_to_run() {
    %s # This will be replaced by scriptWithTimeout
  }
  _script_to_run
} 1> >(tee -a "/tmp/poststart-stdout.txt") 2> >(tee -a "/tmp/poststart-stderr.txt" >&2)
`
)

func AddPostStartLifecycleHooks(wksp *dw.DevWorkspaceTemplateSpec, containers []corev1.Container, postStartTimeout *int32) error {
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

		postStartHandler, err := processCommandsForPostStart(commands, postStartTimeout)
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
func processCommandsForPostStart(commands []dw.Command, postStartTimeout *int32) (*corev1.LifecycleHandler, error) {
	var commandScriptLines []string
	for _, command := range commands {
		execCmd := command.Exec
		if len(execCmd.Env) > 0 {
			return nil, fmt.Errorf("env vars in postStart command %s are unsupported", command.Id)
		}
		var singleCommandParts []string
		if execCmd.WorkingDir != "" {
			// Safely quote the working directory path
			safeWorkingDir := strings.ReplaceAll(execCmd.WorkingDir, "'", `'\''`)
			singleCommandParts = append(singleCommandParts, fmt.Sprintf("cd '%s'", safeWorkingDir))
		}
		if execCmd.CommandLine != "" {
			singleCommandParts = append(singleCommandParts, execCmd.CommandLine)
		}
		if len(singleCommandParts) > 0 {
			commandScriptLines = append(commandScriptLines, strings.Join(singleCommandParts, " && "))
		}
	}

	originalUserScript := strings.Join(commandScriptLines, "\n")

	scriptToExecute := "set -e\n" + originalUserScript
	escapedUserScript := strings.ReplaceAll(scriptToExecute, "'", `'\''`)

	scriptWithTimeout := fmt.Sprintf(`
export POSTSTART_TIMEOUT_DURATION="%d"
export POSTSTART_KILL_AFTER_DURATION="5"

echo "[postStart hook] Executing commands with timeout: ${POSTSTART_TIMEOUT_DURATION} s, kill after: ${POSTSTART_KILL_AFTER_DURATION} s" >&2

# Run the user's script under the 'timeout' utility.
timeout --preserve-status --kill-after="${POSTSTART_KILL_AFTER_DURATION}" "${POSTSTART_TIMEOUT_DURATION}" /bin/sh -c '%s'
exit_code=$?

# Check the exit code from 'timeout'
if [ $exit_code -eq 143 ]; then # 128 + 15 (SIGTERM)
  echo "[postStart hook] Commands terminated by SIGTERM (likely timed out after ${POSTSTART_TIMEOUT_DURATION}s). Exit code 143." >&2
elif [ $exit_code -eq 137 ]; then # 128 + 9 (SIGKILL)
  echo "[postStart hook] Commands forcefully killed by SIGKILL (likely after --kill-after ${POSTSTART_KILL_AFTER_DURATION}s expired). Exit code 137." >&2
elif [ $exit_code -ne 0 ]; then # Catches any other non-zero exit code, including 124
  echo "[postStart hook] Commands failed with exit code $exit_code." >&2
else
  echo "[postStart hook] Commands completed successfully within the time limit." >&2
fi

exit $exit_code
`, *postStartTimeout, escapedUserScript)

	finalScriptForHook := fmt.Sprintf(redirectOutputFmt, scriptWithTimeout)

	handler := &corev1.LifecycleHandler{
		Exec: &corev1.ExecAction{
			Command: []string{
				"/bin/sh",
				"-c",
				finalScriptForHook,
			},
		},
	}
	return handler, nil
}
