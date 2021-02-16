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
	"errors"
	"fmt"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
)

type FakeInternalRegistry struct {
	Plugins map[string]dw.DevWorkspaceTemplate
	Errors  map[string]TestPluginError
}

func (reg *FakeInternalRegistry) IsInInternalRegistry(pluginID string) bool {
	_, pluginOk := reg.Plugins[pluginID]
	_, errOk := reg.Errors[pluginID]
	return pluginOk || errOk
}

func (reg *FakeInternalRegistry) ReadPluginFromInternalRegistry(pluginID string) (*dw.DevWorkspaceTemplate, error) {
	if plugin, ok := reg.Plugins[pluginID]; ok {
		return &plugin, nil
	}
	if err, ok := reg.Errors[pluginID]; ok {
		return nil, errors.New(err.Message)
	}
	return nil, fmt.Errorf("test does not define entry for plugin %s", pluginID)
}
