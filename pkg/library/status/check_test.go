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

package status

import (
	"testing"
)

func TestGetConcisePostStartFailureMessage(t *testing.T) {
	tests := []struct {
		name        string
		kubeletMsg  string
		expectedMsg string
	}{
		{
			name:        "Kubelet internal message - SIGTERM",
			kubeletMsg:  `PostStartHookError: rpc error: code = Unknown desc = command error: command terminated by SIGTERM, message: "[postStart hook] Commands terminated by SIGTERM (likely timed out after 30s). Exit code 143."`,
			expectedMsg: "[postStart hook] Commands terminated by SIGTERM (likely timed out after 30s). Exit code 143.",
		},
		{
			name:        "Kubelet internal message - SIGKILL",
			kubeletMsg:  `PostStartHookError: rpc error: code = Unknown desc = command error: command terminated by SIGKILL, message: "[postStart hook] Commands forcefully killed by SIGKILL (likely after --kill-after 10s expired). Exit code 137."`,
			expectedMsg: "[postStart hook] Commands forcefully killed by SIGKILL (likely after --kill-after 10s expired). Exit code 137.",
		},
		{
			name:        "Kubelet internal message - Generic Fail",
			kubeletMsg:  `PostStartHookError: rpc error: code = Unknown desc = command error: command failed, message: "[postStart hook] Commands failed with exit code 1."`,
			expectedMsg: "[postStart hook] Commands failed with exit code 1.",
		},
		{
			name:        "Kubelet internal message - No match, fall through to Kubelet exit code",
			kubeletMsg:  `PostStartHookError: rpc error: code = Unknown desc = command error: command terminated by signal: SIGTERM, message: "Container command \\\'sleep 60\\\' was terminated by signal SIGTERM"\nexited with 143: ...`,
			expectedMsg: "[postStart hook] Commands terminated by SIGTERM due to timeout",
		},
		{
			name:        "Kubelet reported exit code - 143 (SIGTERM)",
			kubeletMsg:  `PostStartHookError: command 'sh -c ...' exited with 143: ...`,
			expectedMsg: "[postStart hook] Commands terminated by SIGTERM due to timeout",
		},
		{
			name:        "Kubelet exit code - 137 (SIGKILL)",
			kubeletMsg:  `PostStartHookError: command 'sh -c ...' exited with 137: ...`,
			expectedMsg: "[postStart hook] Commands forcefully killed by SIGKILL due to timeout",
		},
		{
			name:        "Kubelet exit code - 1 (Generic)",
			kubeletMsg:  `PostStartHookError: command 'sh -c ...' exited with 1: ...`,
			expectedMsg: "[postStart hook] Commands failed (Kubelet reported exit code 1)",
		},
		{
			name:        "Kubelet exit code - 124 (e.g. timeout command itself)",
			kubeletMsg:  `PostStartHookError: command 'sh -c ...' exited with 124: ...`,
			expectedMsg: "[postStart hook] Commands failed (Kubelet reported exit code 124)",
		},
		{
			name:        "Full Kubelet message match - SIGTERM (no internal message field, no Kubelet exit code first part)",
			kubeletMsg:  `PostStartHookError: Error executing postStart hook: [postStart hook] Commands terminated by SIGTERM (likely timed out after 45s). Exit code 143.`,
			expectedMsg: "[postStart hook] Commands terminated by SIGTERM (likely timed out after 45s). Exit code 143.",
		},
		{
			name:        "Full Kubelet message match - SIGKILL (no internal message field, no Kubelet exit code first part)",
			kubeletMsg:  `PostStartHookError: Error executing postStart hook: [postStart hook] Commands forcefully killed by SIGKILL (likely after --kill-after 5s expired). Exit code 137.`,
			expectedMsg: "[postStart hook] Commands forcefully killed by SIGKILL (likely after --kill-after 5s expired). Exit code 137.",
		},
		{
			name:        "Full Kubelet message match - Generic Fail (no internal message field, no Kubelet exit code first part)",
			kubeletMsg:  `PostStartHookError: Error executing postStart hook: [postStart hook] Commands failed with exit code 2.`,
			expectedMsg: "[postStart hook] Commands failed with exit code 2.",
		},
		{
			name:        "Kubelet internal message with escaped quotes and script output",
			kubeletMsg:  `PostStartHookError: rpc error: code = Unknown desc = failed to exec in container: command /bin/sh -c export POSTSTART_TIMEOUT_DURATION="30s"; export POSTSTART_KILL_AFTER_DURATION="10s"; echo "[postStart hook] Executing user commands with timeout ${POSTSTART_TIMEOUT_DURATION}, kill after ${POSTSTART_KILL_AFTER_DURATION}..."; _script_to_run() { set -e\\necho \\\'hello\\\' >&2\\nexit 1\\n }; timeout --preserve-status --kill-after="${POSTSTART_KILL_AFTER_DURATION}" "${POSTSTART_TIMEOUT_DURATION}" /bin/sh -c "_script_to_run" 1> >(tee -a "/tmp/poststart-stdout.txt") 2> >(tee -a "/tmp/poststart-stderr.txt" >&2); exit_code=$?; if [ $exit_code -eq 143 ]; then echo "[postStart hook] Commands terminated by SIGTERM (likely timed out after ${POSTSTART_TIMEOUT_DURATION}). Exit code 143." >&2; elif [ $exit_code -eq 137 ]; then echo "[postStart hook] Commands forcefully killed by SIGKILL (likely after --kill-after ${POSTSTART_KILL_AFTER_DURATION} expired). Exit code 137." >&2; elif [ $exit_code -ne 0 ]; then echo "[postStart hook] Commands failed with exit code ${exit_code}." >&2; fi; exit $exit_code: exit status 1, message: "[postStart hook] Commands failed with exit code 1."`,
			expectedMsg: "[postStart hook] Commands failed with exit code 1.",
		},
		{
			name:        "Fallback - Unrecognized Kubelet message",
			kubeletMsg:  "PostStartHookError: An unexpected error occurred.",
			expectedMsg: "[postStart hook] failed with an unknown error (see pod events or container logs for more details)",
		},
		{
			name:        "Fallback - Empty Kubelet message",
			kubeletMsg:  "",
			expectedMsg: "[postStart hook] failed with an unknown error (see pod events or container logs for more details)",
		},
		{
			name:        "Kubelet internal message - SIGTERM - with leading/trailing spaces in message",
			kubeletMsg:  `PostStartHookError: rpc error: code = Unknown desc = command error: command terminated by SIGTERM, message: "  [postStart hook] Commands terminated by SIGTERM (likely timed out after 30s). Exit code 143.  "`,
			expectedMsg: "[postStart hook] Commands terminated by SIGTERM (likely timed out after 30s). Exit code 143.",
		},
		{
			name:        "Kubelet exit code - 143 - with surrounding text",
			kubeletMsg:  `FailedPostStartHook: container "theia-ide" postStart hook failed: command 'sh -c mycommand' exited with 143:`,
			expectedMsg: "[postStart hook] Commands terminated by SIGTERM due to timeout",
		},
		{
			name:        "Fallback - Kubelet message with exit code 0 but error text",
			kubeletMsg:  `PostStartHookError: command "sh -c echo hello && exit 0" exited with 0: "unexpected error"`,
			expectedMsg: "[postStart hook] failed with an unknown error (see pod events or container logs for more details)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getConcisePostStartFailureMessage(tt.kubeletMsg); got != tt.expectedMsg {
				t.Errorf("getConcisePostStartFailureMessage() = %v, want %v", got, tt.expectedMsg)
			}
		})
	}
}
