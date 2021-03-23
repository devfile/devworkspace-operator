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
	"regexp"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
)

var SupportedSchemaVersionRegexp = regexp.MustCompile(`^2\..+`)

func ConvertDevfileToDevWorkspaceTemplate(devfile *dw.Devfile) (*dw.DevWorkspaceTemplate, error) {
	if !SupportedSchemaVersionRegexp.MatchString(devfile.SchemaVersion) {
		return nil, fmt.Errorf("could not process devfile: unsupported schemaVersion '%s'", devfile.SchemaVersion)
	}
	dwt := &dw.DevWorkspaceTemplate{}
	dwt.Spec = devfile.DevWorkspaceTemplateSpec
	dwt.Name = devfile.Metadata.Name // TODO: Handle additional devfile metadata once those changes are pulled in to this repo

	return dwt, nil
}
