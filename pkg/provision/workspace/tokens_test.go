// Copyright (c) 2019-2024 Red Hat, Inc.
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

package workspace

import (
	"strings"
	"testing"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

var saTokenDiffOpts = cmp.Options{
	cmpopts.SortSlices(func(a, b corev1.Volume) bool {
		return strings.Compare(a.Name, b.Name) > 0
	}),
	cmpopts.SortSlices(func(a, b corev1.VolumeMount) bool {
		return strings.Compare(a.Name, b.Name) > 0
	}),
}

const (
	permissionBits = 0640
)

func TestServiceAccountTokenProjection(t *testing.T) {
	tests := []struct {
		name                 string
		serviceAccountTokens []v1alpha1.ServiceAccountToken
		expectedVolumes      []corev1.Volume
		expectedVolumeMounts []corev1.VolumeMount
		errRegexp            string
	}{
		{
			name: "Creates a volume and volume mount for a ServiceAccountToken",
			serviceAccountTokens: []v1alpha1.ServiceAccountToken{
				{
					Name:              "test-token",
					MountPath:         "/var/run/secrets/tokens",
					Path:              "test-path",
					Audience:          "openshift",
					ExpirationSeconds: 3600},
			},
			expectedVolumes: []corev1.Volume{
				testServiceAccountTokenProjectionVolume("test-token", []corev1.VolumeProjection{
					testServiceAccountTokenProjection("openshift", "test-path", 3600),
				})},
			expectedVolumeMounts: []corev1.VolumeMount{
				testServiceAccountTokenProjectionVolumeMount("test-token", "/var/run/secrets/tokens"),
			},
		},
		{
			name: "Uses different volumes and volume mounts for ServiceAccount tokens with different mount paths",
			serviceAccountTokens: []v1alpha1.ServiceAccountToken{
				{
					Name:              "test-token-1",
					MountPath:         "/tmp/",
					Path:              "test-path-1",
					Audience:          "openshift",
					ExpirationSeconds: 3600},
				{
					Name:              "test-token-2",
					MountPath:         "/var/run/secrets/tokens",
					Path:              "test-path-2",
					Audience:          "openshift",
					ExpirationSeconds: 7200},
			},
			expectedVolumes: []corev1.Volume{
				testServiceAccountTokenProjectionVolume("test-token-1", []corev1.VolumeProjection{
					testServiceAccountTokenProjection("openshift", "test-path-1", 3600),
				}),
				testServiceAccountTokenProjectionVolume("test-token-2", []corev1.VolumeProjection{
					testServiceAccountTokenProjection("openshift", "test-path-2", 7200),
				})},

			expectedVolumeMounts: []corev1.VolumeMount{
				testServiceAccountTokenProjectionVolumeMount("test-token-1", "/tmp/"),
				testServiceAccountTokenProjectionVolumeMount("test-token-2", "/var/run/secrets/tokens"),
			},
		},
		{
			name: "Uses the same projected volume and same volume mount for ServiceAccount tokens with same mount paths",
			serviceAccountTokens: []v1alpha1.ServiceAccountToken{
				{
					Name:              "test-token-1",
					MountPath:         "/var/run/secrets/tokens",
					Path:              "test-path-1",
					Audience:          "openshift",
					ExpirationSeconds: 3600},
				{
					Name:              "test-token-2",
					MountPath:         "/var/run/secrets/tokens",
					Path:              "test-path-2",
					Audience:          "openshift",
					ExpirationSeconds: 7200},
			},
			expectedVolumes: []corev1.Volume{
				testServiceAccountTokenProjectionVolume(common.ServiceAccountTokenProjectionName("/var/run/secrets/tokens"), []corev1.VolumeProjection{
					testServiceAccountTokenProjection("openshift", "test-path-1", 3600),
					testServiceAccountTokenProjection("openshift", "test-path-2", 7200),
				})},

			expectedVolumeMounts: []corev1.VolumeMount{
				testServiceAccountTokenProjectionVolumeMount(common.ServiceAccountTokenProjectionName("/var/run/secrets/tokens"), "/var/run/secrets/tokens"),
			},
		},
		{
			name: "Detects path collisions when multiple ServiceAccount tokens use the same mount path and path (single collision)",
			serviceAccountTokens: []v1alpha1.ServiceAccountToken{
				{
					Name:              "test-token-1",
					MountPath:         "/var/run/secrets/tokens",
					Path:              "test-path",
					Audience:          "openshift",
					ExpirationSeconds: 3600},
				{
					Name:              "test-token-2",
					MountPath:         "/var/run/secrets/tokens",
					Path:              "test-path",
					Audience:          "openshift",
					ExpirationSeconds: 7200},
			},
			errRegexp: "the following ServiceAccount tokens have the same path (test-path) and mount path (/var/run/secrets/tokens): test-token-1, test-token-2",
		},
		{
			name: "Detects path collisions when multiple ServiceAccount tokens use the same mount path and path (two collisions for different paths)",
			serviceAccountTokens: []v1alpha1.ServiceAccountToken{
				{
					Name:              "test-token-1",
					MountPath:         "/var/run/secrets/tokens",
					Path:              "test-path",
					Audience:          "openshift",
					ExpirationSeconds: 3600},
				{
					Name:              "test-token-2",
					MountPath:         "/var/run/secrets/tokens",
					Path:              "test-path",
					Audience:          "openshift",
					ExpirationSeconds: 7200},
				{
					Name:              "test-token-3",
					MountPath:         "/var/run/secrets/tokens",
					Path:              "other-path",
					Audience:          "openshift",
					ExpirationSeconds: 4000},
				{
					Name:              "test-token-4",
					MountPath:         "/var/run/secrets/tokens",
					Path:              "other-path",
					Audience:          "openshift",
					ExpirationSeconds: 5000},
			},
			errRegexp: "multiple ServiceAccount tokens share the same path and mount path: test-token-1, test-token-2, test-token-3, test-token-4",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualVolumeMounts, actualVolumes, err := getSATokensVolumesAndVolumeMounts(tt.serviceAccountTokens)

			if tt.errRegexp != "" {
				if !assert.Error(t, err) {
					return
				}
				assert.Equal(t, tt.errRegexp, err.Error(), "Error message should match")
			} else {
				if !assert.NoError(t, err, "Should not return error") {
					return
				}
				assert.True(t, cmp.Equal(tt.expectedVolumes, actualVolumes, saTokenDiffOpts), cmp.Diff(tt.expectedVolumes, actualVolumes, saTokenDiffOpts))
				assert.True(t, cmp.Equal(tt.expectedVolumeMounts, actualVolumeMounts, saTokenDiffOpts), cmp.Diff(tt.expectedVolumeMounts, actualVolumeMounts, saTokenDiffOpts))
			}
		})
	}
}

func testServiceAccountTokenProjectionVolumeMount(name, mounthPath string) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      name,
		MountPath: mounthPath,
		ReadOnly:  true,
	}
}

func testServiceAccountTokenProjection(audience, path string, expirationSeconds int64) corev1.VolumeProjection {
	return corev1.VolumeProjection{
		ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
			Audience:          audience,
			ExpirationSeconds: pointer.Int64(expirationSeconds),
			Path:              path,
		},
	}
}

func testServiceAccountTokenProjectionVolume(name string, serviceAccountTokens []corev1.VolumeProjection) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			Projected: &corev1.ProjectedVolumeSource{
				DefaultMode: pointer.Int32(permissionBits),
				Sources:     serviceAccountTokens,
			},
		},
	}
}
