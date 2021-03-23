//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
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
