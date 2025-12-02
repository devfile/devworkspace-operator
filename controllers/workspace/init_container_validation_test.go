//
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
//

package controllers

import (
	"strings"
	"testing"

	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestValidateInitContainer(t *testing.T) {
	tests := []struct {
		name        string
		container   corev1.Container
		expectError bool
		errorMsg    string
	}{
		{
			name: "Accepts container with only args (image and command will be filled by merge)",
			container: corev1.Container{
				Name: constants.HomeInitComponentName,
				Args: []string{"echo 'test'"},
			},
			expectError: false,
		},
		{
			name: "Accepts container with only image (command and args will be filled by merge)",
			container: corev1.Container{
				Name:  constants.HomeInitComponentName,
				Image: "custom-image:latest",
			},
			expectError: false,
		},
		{
			name: "Accepts valid command",
			container: corev1.Container{
				Name:    constants.HomeInitComponentName,
				Image:   "custom-image:latest",
				Command: []string{"/bin/sh", "-c"},
				Args:    []string{"echo 'test'"},
			},
			expectError: false,
		},
		{
			name: "Rejects invalid command",
			container: corev1.Container{
				Name:    constants.HomeInitComponentName,
				Image:   "custom-image:latest",
				Command: []string{"/bin/sh"},
				Args:    []string{"echo 'test'"},
			},
			expectError: true,
			errorMsg:    "command must be exactly [/bin/sh, -c]",
		},
		{
			name: "Rejects empty command",
			container: corev1.Container{
				Name:    constants.HomeInitComponentName,
				Image:   "custom-image:latest",
				Command: []string{},
				Args:    []string{"echo 'test'"},
			},
			expectError: true,
			errorMsg:    "command must be exactly [/bin/sh, -c]",
		},
		{
			name: "Rejects empty args",
			container: corev1.Container{
				Name:  constants.HomeInitComponentName,
				Image: "custom-image:latest",
				Args:  []string{},
			},
			expectError: true,
			errorMsg:    "args must contain exactly one script string",
		},
		{
			name: "Rejects multiple args",
			container: corev1.Container{
				Name:  constants.HomeInitComponentName,
				Image: "custom-image:latest",
				Args:  []string{"echo 'test'", "echo 'test2'"},
			},
			expectError: true,
			errorMsg:    "args must contain exactly one script string",
		},
		{
			name: "Rejects user-provided volumeMounts",
			container: corev1.Container{
				Name:  constants.HomeInitComponentName,
				Image: "custom-image:latest",
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "custom-volume",
						MountPath: "/mnt/custom",
					},
				},
			},
			expectError: true,
			errorMsg:    "volumeMounts are not allowed for init-persistent-home",
		},
		{
			name: "Allows env variables",
			container: corev1.Container{
				Name:  constants.HomeInitComponentName,
				Image: "custom-image:latest",
				Env: []corev1.EnvVar{
					{
						Name:  "TEST_VAR",
						Value: "test-var",
					},
				},
			},
			expectError: false,
		},
		{
			name: "Rejects ports",
			container: corev1.Container{
				Name:  constants.HomeInitComponentName,
				Image: "custom-image:latest",
				Ports: []corev1.ContainerPort{
					{
						ContainerPort: 8080,
					},
				},
			},
			expectError: true,
			errorMsg:    "ports are not allowed",
		},
		{
			name: "Rejects livenessProbe",
			container: corev1.Container{
				Name:  constants.HomeInitComponentName,
				Image: "custom-image:latest",
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/health",
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "probes are not allowed",
		},
		{
			name: "Rejects securityContext",
			container: corev1.Container{
				Name:  constants.HomeInitComponentName,
				Image: "custom-image:latest",
				SecurityContext: &corev1.SecurityContext{
					RunAsUser: new(int64),
				},
			},
			expectError: true,
			errorMsg:    "securityContext is not allowed",
		},
		{
			name: "Rejects resources",
			container: corev1.Container{
				Name:  constants.HomeInitComponentName,
				Image: "custom-image:latest",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
			expectError: true,
			errorMsg:    "resource limits/requests are not allowed",
		},
		{
			name: "Rejects workingDir",
			container: corev1.Container{
				Name:       constants.HomeInitComponentName,
				Image:      "custom-image:latest",
				WorkingDir: "/tmp",
			},
			expectError: true,
			errorMsg:    "workingDir is not allowed",
		},
		{
			name: "Rejects image with whitespace",
			container: corev1.Container{
				Name:  constants.HomeInitComponentName,
				Image: "nginx\tmalicious",
			},
			expectError: true,
			errorMsg:    "invalid image reference",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHomeInitContainer(tt.container)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateImageReference(t *testing.T) {
	tests := []struct {
		name        string
		image       string
		expectError bool
		errorMsg    string
	}{
		// Valid images
		{"simple image", "nginx", false, ""},
		{"image with tag", "nginx:latest", false, ""},
		{"image with version tag", "nginx:1.21", false, ""},
		{"registry image", "docker.io/nginx", false, ""},
		{"registry with tag", "docker.io/nginx:latest", false, ""},
		{"registry with port", "localhost:5000/nginx", false, ""},
		{"registry port with tag", "localhost:5000/nginx:latest", false, ""},
		{"multi-level path", "registry.example.com/team/project/app", false, ""},
		{"with digest", "nginx@sha256:abc123def4567890abcdef1234567890abcdef1234567890abcdef1234567890", false, ""},
		{"full reference", "registry.example.com:8080/team/app:v1.2.3@sha256:abc123def4567890abcdef1234567890abcdef1234567890abcdef1234567890", false, ""},

		// Invalid images
		{"empty image", "", true, "cannot be empty"},
		{"whitespace", "nginx latest", true, "whitespace"},
		{"newline", "nginx\nlatest", true, "whitespace"},
		{"tab", "nginx\tlatest", true, "whitespace"},
		{"control char", "nginx\x00latest", true, "control characters"},
		{"invalid port 0", "registry:0/image", true, "port number"},
		{"invalid port 65536", "registry:65536/image", true, "port number"},
		{"invalid format", "-nginx", true, "invalid format: should match regex: ^([a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])*|\\[?[0-9a-fA-F:]+]?)(:\\d{1,5})?(/[a-zA-Z0-9]([a-zA-Z0-9._/-]*[a-zA-Z0-9])*)*(:[a-zA-Z0-9_.-]+)?(@sha256:[a-f0-9]{64})?$"},
		{"too long", strings.Repeat("a", 4097), true, "exceeds 4096"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateImageReference(tt.image)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err, "Image %q should be valid", tt.image)
			}
		})
	}
}
