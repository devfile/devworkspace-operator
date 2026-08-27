//
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
//

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeVolumeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "replaces dots with hyphens",
			input:    "test.pullsecret",
			expected: "test-pullsecret",
		},
		{
			name:     "replaces all invalid characters and lowercases",
			input:    "Test.Secret_Name@example",
			expected: "test-secret-name-example",
		},
		{
			name:     "collapses consecutive invalid characters and trims edges",
			input:    ".test...secret.",
			expected: "test-secret",
		},
		{
			// Hyphens are non-alphanumeric, so the [^a-z0-9]+ regex matches a run of
			// literal hyphens and collapses it to a single '-'. This guarantees the
			// sanitized name can never contain two or more consecutive hyphens.
			name:     "collapses consecutive literal hyphens into a single hyphen",
			input:    "test--.-secret",
			expected: "test-secret",
		},
		{
			name:     "leaves already valid names unchanged",
			input:    "valid-secret-123",
			expected: "valid-secret-123",
		},
		{
			name:     "truncates to 63 characters",
			input:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-----bb",
			expected: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-b",
		},
		{
			name:     "truncates characters without a trailing hyphen, keeping a valid ending",
			input:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-----bb",
			expected: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeVolumeName(tt.input)
			assert.Equal(t, tt.expected, result, "sanitizeVolumeName(%q) should match expected value", tt.input)

			// Verify DNS-1123 label compliance
			assert.LessOrEqual(t, len(result), 63, "Volume name should not exceed 63 characters")
			assert.Regexp(t, "^[a-z0-9]([-a-z0-9]*[a-z0-9])?$", result, "Volume name should be a valid DNS-1123 label")
		})
	}
}
