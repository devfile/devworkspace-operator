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

package initcontainers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestMergeInitContainers(t *testing.T) {
	tests := []struct {
		name    string
		base    []corev1.Container
		patches []corev1.Container
		want    []corev1.Container
		wantErr bool
	}{
		{
			name: "empty base",
			base: []corev1.Container{},
			patches: []corev1.Container{
				{Name: "new-container", Image: "new-image"},
			},
			want: []corev1.Container{
				{Name: "new-container", Image: "new-image"},
			},
			wantErr: false,
		},
		{
			name: "empty patches",
			base: []corev1.Container{
				{Name: "base-container", Image: "base-image"},
			},
			patches: []corev1.Container{},
			want: []corev1.Container{
				{Name: "base-container", Image: "base-image"},
			},
			wantErr: false,
		},
		{
			name: "multiple containers",
			base: []corev1.Container{
				{Name: "first", Image: "first-image"},
				{Name: "second", Image: "second-image"},
				{Name: "third", Image: "third-image"},
			},
			patches: []corev1.Container{
				{Name: "new-container", Image: "new-image"},
				{Name: "second", Image: "updated-second-image"},
			},
			want: []corev1.Container{
				{Name: "first", Image: "first-image"},
				{Name: "second", Image: "updated-second-image"},
				{Name: "third", Image: "third-image"},
				{Name: "new-container", Image: "new-image"},
			},
			wantErr: false,
		},
		{
			name: "partial field merge",
			base: []corev1.Container{
				{
					Name:    "base-container",
					Image:   "base-image",
					Command: []string{"/bin/sh", "-c"},
					Args:    []string{"echo 'base'"},
					Env:     []corev1.EnvVar{{Name: "BASE_VAR", Value: "base-value"}},
				},
			},
			patches: []corev1.Container{
				{
					Name: "base-container",
					Args: []string{"echo 'patched'"}, // only this field changed
				},
			},
			want: []corev1.Container{
				{
					Name:    "base-container",
					Image:   "base-image",
					Command: []string{"/bin/sh", "-c"},
					Args:    []string{"echo 'patched'"},
					Env:     []corev1.EnvVar{{Name: "BASE_VAR", Value: "base-value"}},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MergeInitContainers(tt.base, tt.patches)
			if tt.wantErr {
				assert.Error(t, err, "should return error")
			} else {
				assert.NoError(t, err, "should not return error")
				assert.Equal(t, tt.want, got, "should return merged containers")
			}
		})
	}
}
