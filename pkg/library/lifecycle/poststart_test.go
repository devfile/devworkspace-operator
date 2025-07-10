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
	"os"
	"path/filepath"
	"testing"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

type postStartTestCase struct {
	Name     string              `json:"name,omitempty"`
	Input    postStartTestInput  `json:"input,omitempty"`
	Output   postStartTestOutput `json:"output,omitempty"`
	testPath string
}

type postStartTestInput struct {
	Devfile    *dw.DevWorkspaceTemplateSpec `json:"devfile,omitempty"`
	Containers []corev1.Container           `json:"containers,omitempty"`
}

type postStartTestOutput struct {
	Containers []corev1.Container `json:"containers,omitempty"`
	ErrRegexp  *string            `json:"errRegexp,omitempty"`
}

func loadPostStartTestCaseOrPanic(t *testing.T, testPath string) postStartTestCase {
	bytes, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatal(err)
	}
	var test postStartTestCase
	if err := yaml.Unmarshal(bytes, &test); err != nil {
		t.Fatal(err)
	}
	test.testPath = testPath
	return test
}

func loadAllPostStartTestCasesOrPanic(t *testing.T, fromDir string) []postStartTestCase {
	files, err := os.ReadDir(fromDir)
	if err != nil {
		t.Fatal(err)
	}
	var tests []postStartTestCase
	for _, file := range files {
		if file.IsDir() {
			tests = append(tests, loadAllPostStartTestCasesOrPanic(t, filepath.Join(fromDir, file.Name()))...)
		} else {
			tests = append(tests, loadPostStartTestCaseOrPanic(t, filepath.Join(fromDir, file.Name())))
		}
	}
	return tests
}

func TestAddPostStartLifecycleHooks(t *testing.T) {
	tests := loadAllPostStartTestCasesOrPanic(t, "./testdata/postStart")
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s (%s)", tt.Name, tt.testPath), func(t *testing.T) {
			err := AddPostStartLifecycleHooks(tt.Input.Devfile, tt.Input.Containers, nil)
			if tt.Output.ErrRegexp != nil && assert.Error(t, err) {
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error(), "Error message should match")
			} else {
				if !assert.NoError(t, err, "Should not return error") {
					return
				}
				assert.Equal(t, tt.Output.Containers, tt.Input.Containers, "Containers should be updated to match expected output")
			}
		})
	}
}

func TestBuildUserScript(t *testing.T) {
	tests := []struct {
		name           string
		commands       []dw.Command
		expectedScript string
		expectedErr    string
	}{
		{
			name:           "No commands",
			commands:       []dw.Command{},
			expectedScript: "",
			expectedErr:    "",
		},
		{
			name: "Single command without workingDir",
			commands: []dw.Command{
				{
					Id: "cmd1",
					CommandUnion: dw.CommandUnion{
						Exec: &dw.ExecCommand{
							CommandLine: "echo hello",
							Component:   "tools",
						},
					},
				},
			},
			expectedScript: "echo hello",
			expectedErr:    "",
		},
		{
			name: "Single command with workingDir",
			commands: []dw.Command{
				{
					Id: "cmd1",
					CommandUnion: dw.CommandUnion{
						Exec: &dw.ExecCommand{
							CommandLine: "ls -la",
							WorkingDir:  "/projects/app",
							Component:   "tools",
						},
					},
				},
			},
			expectedScript: "cd '/projects/app' && ls -la",
			expectedErr:    "",
		},
		{
			name: "Single command with only workingDir",
			commands: []dw.Command{
				{
					Id: "cmd1",
					CommandUnion: dw.CommandUnion{
						Exec: &dw.ExecCommand{
							WorkingDir: "/data",
							Component:  "tools",
						},
					},
				},
			},
			expectedScript: "cd '/data'",
			expectedErr:    "",
		},
		{
			name: "Single command with workingDir containing single quote",
			commands: []dw.Command{
				{
					Id: "cmd1",
					CommandUnion: dw.CommandUnion{
						Exec: &dw.ExecCommand{
							CommandLine: "cat file.txt",
							WorkingDir:  "/projects/app's",
							Component:   "tools",
						},
					},
				},
			},
			expectedScript: "cd '/projects/app'\\''s' && cat file.txt",
			expectedErr:    "",
		},
		{
			name: "Multiple commands",
			commands: []dw.Command{
				{
					Id: "cmd1",
					CommandUnion: dw.CommandUnion{
						Exec: &dw.ExecCommand{
							CommandLine: "npm install",
							WorkingDir:  "/projects/frontend",
							Component:   "tools",
						},
					},
				},
				{
					Id: "cmd2",
					CommandUnion: dw.CommandUnion{
						Exec: &dw.ExecCommand{
							CommandLine: "npm start",
							Component:   "tools",
						},
					},
				},
				{
					Id: "cmd3",
					CommandUnion: dw.CommandUnion{
						Exec: &dw.ExecCommand{
							WorkingDir:  "/projects/backend",
							CommandLine: "mvn spring-boot:run",
							Component:   "tools",
						},
					},
				},
			},
			expectedScript: "cd '/projects/frontend' && npm install\nnpm start\ncd '/projects/backend' && mvn spring-boot:run",
			expectedErr:    "",
		},
		{
			name: "Command with unsupported Env vars",
			commands: []dw.Command{
				{
					Id: "cmd-with-env",
					CommandUnion: dw.CommandUnion{
						Exec: &dw.ExecCommand{
							CommandLine: "echo $MY_VAR",
							Component:   "tools",
							Env: []dw.EnvVar{
								{Name: "MY_VAR", Value: "test"},
							},
						},
					},
				},
			},
			expectedScript: "",
			expectedErr:    "env vars in postStart command cmd-with-env are unsupported",
		},
		{
			name: "Command with nil Exec field",
			commands: []dw.Command{
				{
					Id:           "cmd-nil-exec",
					CommandUnion: dw.CommandUnion{Exec: nil},
				},
			},
			expectedScript: "",
			expectedErr:    "exec command is nil for command ID cmd-nil-exec",
		},
		{
			name: "Command with empty CommandLine and no WorkingDir",
			commands: []dw.Command{
				{
					Id: "cmd-empty",
					CommandUnion: dw.CommandUnion{
						Exec: &dw.ExecCommand{
							CommandLine: "",
							WorkingDir:  "",
							Component:   "tools",
						},
					},
				},
				{
					Id: "cmd-after-empty",
					CommandUnion: dw.CommandUnion{
						Exec: &dw.ExecCommand{
							CommandLine: "echo 'still works'",
							Component:   "tools",
						},
					},
				},
			},
			expectedScript: "echo 'still works'", // The empty command should result in no line
			expectedErr:    "",
		},
		{
			name: "Command with only CommandLine (empty working dir)",
			commands: []dw.Command{
				{
					Id: "cmd-empty-wdir",
					CommandUnion: dw.CommandUnion{
						Exec: &dw.ExecCommand{
							CommandLine: "pwd",
							WorkingDir:  "",
							Component:   "tools",
						},
					},
				},
			},
			expectedScript: "pwd",
			expectedErr:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script, err := buildUserScript(tt.commands)

			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedScript, script)
			}
		})
	}
}

func TestGenerateScriptWithTimeout(t *testing.T) {
	tests := []struct {
		name              string
		escapedUserScript string
		timeoutSeconds    int32
		expectedScript    string
	}{
		{
			name:              "Basic script with timeout",
			escapedUserScript: "echo 'hello world'\nsleep 1",
			timeoutSeconds:    10,
			expectedScript: `
export POSTSTART_TIMEOUT_DURATION="10"
export POSTSTART_KILL_AFTER_DURATION="5"

_TIMEOUT_COMMAND_PART=""
_WAS_TIMEOUT_USED="false" # Use strings "true" or "false" for shell boolean

if command -v timeout >/dev/null 2>&1; then
  echo "[postStart hook] Executing commands with timeout: ${POSTSTART_TIMEOUT_DURATION} seconds, kill after: ${POSTSTART_KILL_AFTER_DURATION} seconds" >&2
  _TIMEOUT_COMMAND_PART="timeout --preserve-status --kill-after=\"${POSTSTART_KILL_AFTER_DURATION}\" \"${POSTSTART_TIMEOUT_DURATION}\""
  _WAS_TIMEOUT_USED="true"
else
  echo "[postStart hook] WARNING: 'timeout' utility not found. Executing commands without timeout." >&2
fi

# Execute the user's script
${_TIMEOUT_COMMAND_PART} /bin/sh -c 'echo 'hello world'
sleep 1'
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
`,
		},
		{
			name:              "Script with zero timeout (no timeout)",
			escapedUserScript: "echo 'running indefinitely...'",
			timeoutSeconds:    0,
			expectedScript: `
export POSTSTART_TIMEOUT_DURATION="0"
export POSTSTART_KILL_AFTER_DURATION="5"

_TIMEOUT_COMMAND_PART=""
_WAS_TIMEOUT_USED="false" # Use strings "true" or "false" for shell boolean

if command -v timeout >/dev/null 2>&1; then
  echo "[postStart hook] Executing commands with timeout: ${POSTSTART_TIMEOUT_DURATION} seconds, kill after: ${POSTSTART_KILL_AFTER_DURATION} seconds" >&2
  _TIMEOUT_COMMAND_PART="timeout --preserve-status --kill-after=\"${POSTSTART_KILL_AFTER_DURATION}\" \"${POSTSTART_TIMEOUT_DURATION}\""
  _WAS_TIMEOUT_USED="true"
else
  echo "[postStart hook] WARNING: 'timeout' utility not found. Executing commands without timeout." >&2
fi

# Execute the user's script
${_TIMEOUT_COMMAND_PART} /bin/sh -c 'echo 'running indefinitely...''
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
`,
		},
		{
			name:              "Empty user script",
			escapedUserScript: "",
			timeoutSeconds:    5,
			expectedScript: `
export POSTSTART_TIMEOUT_DURATION="5"
export POSTSTART_KILL_AFTER_DURATION="5"

_TIMEOUT_COMMAND_PART=""
_WAS_TIMEOUT_USED="false" # Use strings "true" or "false" for shell boolean

if command -v timeout >/dev/null 2>&1; then
  echo "[postStart hook] Executing commands with timeout: ${POSTSTART_TIMEOUT_DURATION} seconds, kill after: ${POSTSTART_KILL_AFTER_DURATION} seconds" >&2
  _TIMEOUT_COMMAND_PART="timeout --preserve-status --kill-after=\"${POSTSTART_KILL_AFTER_DURATION}\" \"${POSTSTART_TIMEOUT_DURATION}\""
  _WAS_TIMEOUT_USED="true"
else
  echo "[postStart hook] WARNING: 'timeout' utility not found. Executing commands without timeout." >&2
fi

# Execute the user's script
${_TIMEOUT_COMMAND_PART} /bin/sh -c ''
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
`,
		},
		{
			name:              "User script with already escaped single quotes",
			escapedUserScript: "echo 'it'\\''s complex'",
			timeoutSeconds:    30,
			expectedScript: `
export POSTSTART_TIMEOUT_DURATION="30"
export POSTSTART_KILL_AFTER_DURATION="5"

_TIMEOUT_COMMAND_PART=""
_WAS_TIMEOUT_USED="false" # Use strings "true" or "false" for shell boolean

if command -v timeout >/dev/null 2>&1; then
  echo "[postStart hook] Executing commands with timeout: ${POSTSTART_TIMEOUT_DURATION} seconds, kill after: ${POSTSTART_KILL_AFTER_DURATION} seconds" >&2
  _TIMEOUT_COMMAND_PART="timeout --preserve-status --kill-after=\"${POSTSTART_KILL_AFTER_DURATION}\" \"${POSTSTART_TIMEOUT_DURATION}\""
  _WAS_TIMEOUT_USED="true"
else
  echo "[postStart hook] WARNING: 'timeout' utility not found. Executing commands without timeout." >&2
fi

# Execute the user's script
${_TIMEOUT_COMMAND_PART} /bin/sh -c 'echo 'it'\''s complex''
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
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := generateScriptWithTimeout(tt.escapedUserScript, tt.timeoutSeconds)
			assert.Equal(t, tt.expectedScript, script)
		})
	}
}
