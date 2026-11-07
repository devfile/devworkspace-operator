// Copyright (c) 2019-2026 Red Hat, Inc.
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
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
)

func TestCheckPodEvents(t *testing.T) {
	const (
		workspaceID        = "workspace-id"
		podName            = "workspace-pod"
		namespace          = "workspace-namespace"
		aggregateSecret    = "workspace-id-service"
		aggregateVolume    = "devworkspace-serving-cert-workspace-id-service"
		discoverableSecret = "api"
		discoverableVolume = "devworkspace-serving-cert-api"
		longSecret         = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		longVolume         = "devworkspace-cert-e33cdf9c7f7120b98e8c"
	)

	secretVolume := func(name, secretName string) corev1.Volume {
		return corev1.Volume{
			Name: name,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: secretName},
			},
		}
	}
	failedMount := func(volumeName string, count int32, seriesCount *int32) corev1.Event {
		series := (*corev1.EventSeries)(nil)
		if seriesCount != nil {
			series = &corev1.EventSeries{Count: *seriesCount}
		}
		return corev1.Event{
			ObjectMeta: metav1.ObjectMeta{Name: "failed-mount", Namespace: namespace},
			InvolvedObject: corev1.ObjectReference{
				Kind: "Pod",
				Name: podName,
			},
			Reason:  "FailedMount",
			Message: fmt.Sprintf("MountVolume.SetUp failed for volume %q: secret not found", volumeName),
			Count:   count,
			Series:  series,
		}
	}

	count2, count3 := int32(2), int32(3)
	tests := []struct {
		name           string
		infrastructure infrastructure.Type
		event          corev1.Event
		volumes        []corev1.Volume
		wantFailure    bool
	}{
		{
			name:           "aggregate serving certificate remains ignored on OpenShift",
			infrastructure: infrastructure.OpenShiftv4,
			event:          failedMount(aggregateVolume, 3, nil),
			volumes:        []corev1.Volume{secretVolume(aggregateVolume, aggregateSecret)},
		},
		{
			name:           "discoverable serving certificate is ignored on OpenShift",
			infrastructure: infrastructure.OpenShiftv4,
			event:          failedMount(discoverableVolume, 3, nil),
			volumes:        []corev1.Volume{secretVolume(discoverableVolume, discoverableSecret)},
		},
		{
			name:           "hashed long serving certificate volume is ignored on OpenShift",
			infrastructure: infrastructure.OpenShiftv4,
			event:          failedMount(longVolume, 3, nil),
			volumes:        []corev1.Volume{secretVolume(longVolume, longSecret)},
		},
		{
			name:           "unrelated Secret volume remains fatal",
			infrastructure: infrastructure.OpenShiftv4,
			event:          failedMount("user-secret", 3, nil),
			volumes:        []corev1.Volume{secretVolume("user-secret", "user-secret")},
			wantFailure:    true,
		},
		{
			name:           "similar volume name is not exempted",
			infrastructure: infrastructure.OpenShiftv4,
			event:          failedMount(discoverableVolume+"-extra", 3, nil),
			volumes:        []corev1.Volume{secretVolume(discoverableVolume, discoverableSecret)},
			wantFailure:    true,
		},
		{
			name:           "absent pod volume is not exempted",
			infrastructure: infrastructure.OpenShiftv4,
			event:          failedMount(discoverableVolume, 3, nil),
			wantFailure:    true,
		},
		{
			name:           "volume mapped to another Secret is not exempted",
			infrastructure: infrastructure.OpenShiftv4,
			event:          failedMount(discoverableVolume, 3, nil),
			volumes:        []corev1.Volume{secretVolume(discoverableVolume, "other-secret")},
			wantFailure:    true,
		},
		{
			name:           "volume without a Secret source is not exempted",
			infrastructure: infrastructure.OpenShiftv4,
			event:          failedMount(discoverableVolume, 3, nil),
			volumes:        []corev1.Volume{{Name: discoverableVolume}},
			wantFailure:    true,
		},
		{
			name:           "volume with an empty Secret name is not exempted",
			infrastructure: infrastructure.OpenShiftv4,
			event:          failedMount(discoverableVolume, 3, nil),
			volumes:        []corev1.Volume{secretVolume(discoverableVolume, "")},
			wantFailure:    true,
		},
		{
			name:           "generated certificate failure remains fatal on Kubernetes",
			infrastructure: infrastructure.Kubernetes,
			event:          failedMount(discoverableVolume, 3, nil),
			volumes:        []corev1.Volume{secretVolume(discoverableVolume, discoverableSecret)},
			wantFailure:    true,
		},
		{
			name:           "other unrecoverable event remains fatal on OpenShift",
			infrastructure: infrastructure.OpenShiftv4,
			event: corev1.Event{
				ObjectMeta:     metav1.ObjectMeta{Name: "failed-scheduling", Namespace: namespace},
				InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: podName},
				Reason:         "FailedScheduling",
				Message:        "no nodes available",
			},
			volumes:     []corev1.Volume{secretVolume(discoverableVolume, discoverableSecret)},
			wantFailure: true,
		},
		{
			name:           "legacy Count below threshold remains recoverable",
			infrastructure: infrastructure.OpenShiftv4,
			event:          failedMount("user-secret", 2, nil),
			wantFailure:    false,
		},
		{
			name:           "legacy Count at threshold is fatal",
			infrastructure: infrastructure.OpenShiftv4,
			event:          failedMount("user-secret", 3, nil),
			wantFailure:    true,
		},
		{
			name:           "Series Count takes precedence below threshold",
			infrastructure: infrastructure.OpenShiftv4,
			event:          failedMount("user-secret", 3, &count2),
			wantFailure:    false,
		},
		{
			name:           "Series Count at threshold is fatal",
			infrastructure: infrastructure.OpenShiftv4,
			event:          failedMount("user-secret", 1, &count3),
			wantFailure:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			infrastructure.InitializeForTesting(tt.infrastructure)
			scheme := runtime.NewScheme()
			if err := corev1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			kubeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithIndex(&corev1.Event{}, "involvedObject.name", func(obj client.Object) []string {
					return []string{obj.(*corev1.Event).InvolvedObject.Name}
				}).
				WithObjects(&tt.event).
				Build()
			clusterAPI := sync.ClusterAPI{Client: kubeClient, Ctx: context.Background()}
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: namespace},
				Spec:       corev1.PodSpec{Volumes: tt.volumes},
			}

			msg, err := CheckPodEvents(pod, workspaceID, nil, clusterAPI)
			if err != nil {
				t.Fatalf("CheckPodEvents() error = %v", err)
			}
			if gotFailure := msg != ""; gotFailure != tt.wantFailure {
				t.Errorf("CheckPodEvents() message = %q, want failure %t", msg, tt.wantFailure)
			}
		})
	}
}

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
