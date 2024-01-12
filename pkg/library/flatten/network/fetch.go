//
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
//

package network

import (
	"fmt"
	"io"
	"net/http"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"sigs.k8s.io/yaml"
)

type HTTPGetter interface {
	Get(location string) (*http.Response, error)
}

func FetchDevWorkspaceTemplate(location string, httpClient HTTPGetter) (*dw.DevWorkspaceTemplateSpec, error) {
	resp, err := httpClient.Get(location)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch file from %s: %w", location, err)
	}
	defer resp.Body.Close() // ignoring error because what would we even do?
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("could not fetch file from %s: got status %d", location, resp.StatusCode)
	}
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read data from %s: %w", location, err)
	}

	devfile := &dw.Devfile{}
	if err := yaml.Unmarshal(bytes, devfile); err != nil {
		return nil, fmt.Errorf("could not unmarshal devfile from response: %w", err)
	}
	if devfile.SchemaVersion != "" {
		dwt, err := ConvertDevfileToDevWorkspaceTemplate(devfile)
		if err != nil {
			return nil, fmt.Errorf("failed to convert devfile to DevWorkspaceTemplate: %s", err)
		}
		return &dwt.Spec, nil
	}

	// Assume we didn't get a devfile, check if content is DevWorkspace
	devworkspace := &dw.DevWorkspace{}
	if err := yaml.Unmarshal(bytes, devworkspace); err != nil {
		return nil, fmt.Errorf("could not unmarshal devworkspace from response: %w", err)
	}
	if devworkspace.Kind == "DevWorkspace" {
		return &devworkspace.Spec.Template, nil
	}

	// Check if content is DevWorkspaceTemplate
	dwt := &dw.DevWorkspaceTemplate{}
	if err := yaml.Unmarshal(bytes, dwt); err != nil {
		return nil, fmt.Errorf("could not unmarshal devworkspacetemplate from response: %w", err)
	}
	if dwt.Kind == "DevWorkspaceTemplate" {
		return &dwt.Spec, nil
	}

	return nil, fmt.Errorf("could not find devfile or devworkspace object at '%s'", location)
}
