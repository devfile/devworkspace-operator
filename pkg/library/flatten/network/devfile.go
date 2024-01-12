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
