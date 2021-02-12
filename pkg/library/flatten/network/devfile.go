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

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	devfilev2 "github.com/devfile/api/v2/pkg/devfile"
)

const (
	SupportedDevfileSchemaVersion = "2.0.0" // TODO what should this actually be at this point?
)

type Devfile struct {
	devfilev2.DevfileHeader
	dw.DevWorkspaceTemplateSpec
}

func ConvertDevfileToDevWorkspaceTemplate(devfile *Devfile) (*dw.DevWorkspaceTemplate, error) {
	if devfile.SchemaVersion != SupportedDevfileSchemaVersion {
		return nil, fmt.Errorf("could not process devfile: supported schemaVersion is %s", SupportedDevfileSchemaVersion)
	}
	dwt := &dw.DevWorkspaceTemplate{}
	dwt.Spec = devfile.DevWorkspaceTemplateSpec
	dwt.Name = devfile.Metadata.Name // TODO: Handle additional devfile metadata once those changes are pulled in to this repo

	return dwt, nil
}
