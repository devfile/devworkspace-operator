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

package network

import (
	"fmt"
	"io/ioutil"
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
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read data from %s: %w", location, err)
	}

	// Assume we're getting a devfile, not a DevWorkspaceTemplate (TODO: Detect type and handle both?)
	devfile := &Devfile{}
	err = yaml.Unmarshal(bytes, devfile)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal devfile from response: %w", err)
	}

	dwt, err := ConvertDevfileToDevWorkspaceTemplate(devfile)
	if err != nil {
		return nil, fmt.Errorf("failed to convert devfile to DevWorkspaceTemplate: %s", err)
	}

	return &dwt.Spec, nil
}
