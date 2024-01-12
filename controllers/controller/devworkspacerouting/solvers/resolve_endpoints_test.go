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

package solvers

import (
	"testing"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestGetUrlForEndpoint(t *testing.T) {
	tests := []struct {
		name string

		host         string
		basePath     string
		endpointPath string
		secure       bool

		outURL string
		outErr error
	}{
		{
			name:         "Resolves simple URL with no path components",
			host:         "example.com",
			basePath:     "",
			endpointPath: "",
			secure:       false,

			outURL: "http://example.com",
		},
		{
			name:         "Resolves secure URL with no path components",
			host:         "example.com",
			basePath:     "",
			endpointPath: "",
			secure:       true,

			outURL: "https://example.com",
		},
		{
			name:         "Resolves URL with basepath component, including trailing slash",
			host:         "example.com",
			basePath:     "/test/path/",
			endpointPath: "",
			secure:       true,

			outURL: "https://example.com/test/path/",
		},
		{
			name:         "Resolves URL with basepath component",
			host:         "example.com",
			basePath:     "/test/path",
			endpointPath: "",
			secure:       true,

			outURL: "https://example.com/test/path",
		},
		{
			name:         "Resolves URL with absolute endpoint path component",
			host:         "example.com",
			basePath:     "",
			endpointPath: "/endpoint/path/",
			secure:       true,

			outURL: "https://example.com/endpoint/path/",
		},
		{
			name:         "Resolves URL with endpoint and base path components",
			host:         "example.com",
			basePath:     "base/path/",
			endpointPath: "endpoint/path/",
			secure:       true,

			outURL: "https://example.com/base/path/endpoint/path/",
		},
		{
			name:         "Resolves URL with query param in endpoint path",
			host:         "example.com",
			basePath:     "",
			endpointPath: "?test=param",
			secure:       true,

			outURL: "https://example.com/?test=param",
		},
		{
			name:         "Resolves URL with query param in endpoint path and base path with trailing slash",
			host:         "example.com",
			basePath:     "/base/path/",
			endpointPath: "?test=param",
			secure:       true,

			outURL: "https://example.com/base/path/?test=param",
		},
		{
			name:         "Resolves URL with query param in endpoint path and base path",
			host:         "example.com",
			basePath:     "/base/path",
			endpointPath: "?test=param",
			secure:       true,

			outURL: "https://example.com/base/path?test=param",
		},
		{
			name:         "Resolves URL with query param and path in endpoint path",
			host:         "example.com",
			basePath:     "base/path/",
			endpointPath: "endpoint/path?test=param",
			secure:       true,

			outURL: "https://example.com/base/path/endpoint/path?test=param",
		},
		{
			name:         "Resolves URL with fragment in endpoint path",
			host:         "example.com",
			basePath:     "base/path/",
			endpointPath: "endpoint/path#test",
			secure:       true,

			outURL: "https://example.com/base/path/endpoint/path#test",
		},
		{
			name:         "Resolves URL with absolute endpoint and base path components",
			host:         "example.com",
			basePath:     "/base/path/",
			endpointPath: "/endpoint/path",
			secure:       true,

			outURL: "https://example.com/base/path/endpoint/path",
		},
		{
			name:         "Resolves URL with absolute path and query param",
			host:         "example.com",
			basePath:     "/base/path/",
			endpointPath: "/?query=param",
			secure:       true,

			outURL: "https://example.com/base/path/?query=param",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoint := controllerv1alpha1.Endpoint{
				Protocol: "http",
				Path:     tt.endpointPath,
				Secure:   true,
			}
			url, err := getURLForEndpoint(endpoint, tt.host, tt.basePath, tt.secure)
			if tt.outErr != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.outErr)
			} else {
				if !assert.NoError(t, err) {
					return
				}
				assert.Equal(t, tt.outURL, url)
			}
		})
	}
}
