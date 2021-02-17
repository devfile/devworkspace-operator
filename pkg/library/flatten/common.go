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

package flatten

import devworkspace "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"

func DevWorkspaceIsFlattened(devworkspace devworkspace.DevWorkspaceTemplateSpec) bool {
	if devworkspace.Parent != nil {
		return false
	}
	for _, component := range devworkspace.Components {
		if component.Plugin != nil {
			return false
		}
	}
	return true
}
