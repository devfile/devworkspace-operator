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

package testutil

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"sigs.k8s.io/yaml"

	"github.com/devfile/devworkspace-operator/pkg/library/flatten/network"
)

type FakeHTTPGetter struct {
	DevfileResources      map[string]dw.Devfile
	DevWorkspaceResources map[string]dw.DevWorkspaceTemplate
	Errors                map[string]TestPluginError
}

var _ network.HTTPGetter = (*FakeHTTPGetter)(nil)

type fakeRespBody struct {
	io.Reader
}

func (_ *fakeRespBody) Close() error { return nil }

func (reg *FakeHTTPGetter) Get(location string) (*http.Response, error) {
	if plugin, ok := reg.DevfileResources[location]; ok {
		yamlBytes, err := yaml.Marshal(plugin)
		if err != nil {
			return nil, fmt.Errorf("error marshalling plugin in test: %w", err)
		}
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       &fakeRespBody{bytes.NewBuffer(yamlBytes)},
		}
		return resp, nil
	}
	if plugin, ok := reg.DevWorkspaceResources[location]; ok {
		yamlBytes, err := yaml.Marshal(plugin)
		if err != nil {
			return nil, fmt.Errorf("error marshalling plugin in test: %w", err)
		}
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       &fakeRespBody{bytes.NewBuffer(yamlBytes)},
		}
		return resp, nil
	}

	if err, ok := reg.Errors[location]; ok {
		if err.StatusCode != 0 {
			return &http.Response{
				StatusCode: err.StatusCode,
				Body:       &fakeRespBody{bytes.NewBuffer([]byte{})},
			}, nil
		}
		return nil, errors.New(err.Message)
	}
	return nil, fmt.Errorf("test does not define entry for plugin at URL %s", location)
}
