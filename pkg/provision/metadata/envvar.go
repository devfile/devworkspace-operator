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

package metadata

import (
	"path"

	corev1 "k8s.io/api/core/v1"
)

const (
	// metadataMountPathEnvVar is the name of an env var added to all containers to specify where workspace yamls are mounted.
	metadataMountPathEnvVar = "DEVWORKSPACE_METADATA"

	// flattenedDevfileMountPathEnvVar is an environment variable holding the path to the flattened devworkspace template spec
	flattenedDevfileMountPathEnvVar = "DEVWORKSPACE_FLATTENED_DEVFILE"

	// originalDevfileMountPathEnvVar is an environment variable holding the path to the original devworkspace template spec
	originalDevfileMountPathEnvVar = "DEVWORKSPACE_ORIGINAL_DEVFILE"
)

func getWorkspaceMetaEnvVar() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  metadataMountPathEnvVar,
			Value: metadataMountPath,
		},
		{
			Name:  flattenedDevfileMountPathEnvVar,
			Value: path.Join(metadataMountPath, flattenedYamlFilename),
		},
		{
			Name:  originalDevfileMountPathEnvVar,
			Value: path.Join(metadataMountPath, originalYamlFilename),
		},
	}
}
