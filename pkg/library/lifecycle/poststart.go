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
	"time"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
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

	noTimeoutRedirectOutputFmt = `{
%s
} 1>/tmp/poststart-stdout.txt 2>/tmp/poststart-stderr.txt
`
)

func AddPostStartLifecycleHooks(wksp *dw.DevWorkspaceTemplateSpec, containers []corev1.Container, postStartTimeout string) error {
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

// processCommandsForPostStart processes a list of DevWorkspace commands
// and generates a corev1.LifecycleHandler for the PostStart lifecycle hook.
func processCommandsForPostStart(commands []dw.Command, postStartTimeout string) (*corev1.LifecycleHandler, error) {
	if postStartTimeout == "" {
		// use the fallback if no timeout propagated
		return processCommandsWithoutTimeoutFallback(commands)
	}

	originalUserScript, err := buildUserScript(commands)
	if err != nil {
		return nil, fmt.Errorf("failed to build aggregated user script: %w", err)
	}

	// The user script needs 'set -e' to ensure it exits on error.
	// This script is then passed to `sh -c '...'`, so single quotes within it must be escaped.
	scriptToExecute := "set -e\n" + originalUserScript
	escapedUserScriptForTimeoutWrapper := strings.ReplaceAll(scriptToExecute, "'", `'\''`)

	fullScriptWithTimeout := generateScriptWithTimeout(escapedUserScriptForTimeoutWrapper, postStartTimeout)

	finalScriptForHook := fmt.Sprintf(redirectOutputFmt, fullScriptWithTimeout)

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

// processCommandsWithoutTimeoutFallback builds a lifecycle handler that runs the provided command(s)
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
func processCommandsWithoutTimeoutFallback(commands []dw.Command) (*corev1.LifecycleHandler, error) {
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

	joinedCommands := strings.Join(dwCommands, "\n")

	handler := &corev1.LifecycleHandler{
		Exec: &corev1.ExecAction{
			Command: []string{
				"/bin/sh",
				"-c",
				fmt.Sprintf(noTimeoutRedirectOutputFmt, joinedCommands),
			},
		},
	}
	return handler, nil
}

// buildUserScript takes a list of DevWorkspace commands and constructs a single
// shell script string that executes them sequentially.
func buildUserScript(commands []dw.Command) (string, error) {
	var commandScriptLines []string
	for _, command := range commands {
		execCmd := command.Exec
		if execCmd == nil {
			// Should be caught by earlier validation, but good to be safe
			return "", fmt.Errorf("exec command is nil for command ID %s", command.Id)
		}
		var singleCommandParts []string
		for _, envVar := range execCmd.Env {
			singleCommandParts = append(singleCommandParts, fmt.Sprintf("export %s=%q", envVar.Name, envVar.Value))
		}

		if execCmd.WorkingDir != "" {
			singleCommandParts = append(singleCommandParts, fmt.Sprintf("cd %q", execCmd.WorkingDir))
		}
		if execCmd.CommandLine != "" {
			singleCommandParts = append(singleCommandParts, execCmd.CommandLine)
		}
		if len(singleCommandParts) > 0 {
			commandScriptLines = append(commandScriptLines, strings.Join(singleCommandParts, " && "))
		}
	}
	return strings.Join(commandScriptLines, "\n"), nil
}

// generateScriptWithTimeout wraps a given user script with timeout logic,
// environment variable exports, and specific exit code handling.
// The killAfterDurationSeconds is hardcoded to 5s within this generated script.
// It conditionally prefixes the user script with the timeout command if available.
func generateScriptWithTimeout(escapedUserScript string, postStartTimeout string) string {
	// Convert `postStartTimeout` into the `timeout` format
	var timeoutSeconds int64
	if postStartTimeout != "" && postStartTimeout != "0" {
		duration, err := time.ParseDuration(postStartTimeout)
		if err != nil {
			log.Log.Error(err, "Could not parse post-start timeout, disabling timeout", "value", postStartTimeout)
			timeoutSeconds = 0
		} else {
			timeoutSeconds = int64(duration.Seconds())
		}
	}

	return fmt.Sprintf(`
export POSTSTART_TIMEOUT_DURATION="%d"
export POSTSTART_KILL_AFTER_DURATION="5"

_TIMEOUT_COMMAND_PART=""
_WAS_TIMEOUT_USED="false" # Use strings "true" or "false" for shell boolean

if command -v timeout >/dev/null 2>&1; then
  echo "[postStart hook] Executing commands with timeout: ${POSTSTART_TIMEOUT_DURATION} seconds, kill after: ${POSTSTART_KILL_AFTER_DURATION} seconds" >&2
  _TIMEOUT_COMMAND_PART="timeout --preserve-status --kill-after=${POSTSTART_KILL_AFTER_DURATION} ${POSTSTART_TIMEOUT_DURATION}"
  _WAS_TIMEOUT_USED="true"
else
  echo "[postStart hook] WARNING: 'timeout' utility not found. Executing commands without timeout." >&2
fi

# Execute the user's script
${_TIMEOUT_COMMAND_PART} /bin/sh -c '%s'
exit_code=$?

# Check the exit code based on whether timeout was attempted
if [ "$_WAS_TIMEOUT_USED" = "true" ]; then
  if [ $exit_code -eq 143 ]; then # 128 + 15 (SIGTERM)
    echo "[postStart hook] Commands terminated by SIGTERM (likely timed out after ${POSTSTART_TIMEOUT_DURATION}s). Exit code 143." >&2
  elif [ $exit_code -eq 137 ]; then # 128 + 9 (SIGKILL)
    echo "[postStart hook] Commands forcefully killed by SIGKILL (likely after --kill-after ${POSTSTART_KILL_AFTER_DURATION}s expired). Exit code 137." >&2
  elif [ $exit_code -ne 0 ]; then # Catches any other non-zero exit code
    echo "[postStart hook] Commands failed with exit code $exit_code." >&2
  else
    echo "[postStart hook] Commands completed successfully within the time limit." >&2
  fi
else
  if [ $exit_code -ne 0 ]; then
    echo "[postStart hook] Commands failed with exit code $exit_code (no timeout)." >&2
  else
    echo "[postStart hook] Commands completed successfully (no timeout)." >&2
  fi
fi

exit $exit_code
`, timeoutSeconds, escapedUserScript)
}
