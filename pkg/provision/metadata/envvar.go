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

package metadata

import (
	"path"

	corev1 "k8s.io/api/core/v1"
)

const (
	// MetadataMountPathEnvVar is the name of an env var added to all containers to specify where workspace yamls are mounted.
	MetadataMountPathEnvVar = "DEVWORKSPACE_METADATA"

	// FlattenedDevfileMountPathEnvVar is an environment variable holding the path to the flattened devworkspace template spec
	FlattenedDevfileMountPathEnvVar = "DEVWORKSPACE_FLATTENED_DEVFILE"

	// OriginalDevfileMountPathEnvVar is an environment variable holding the path to the original devworkspace template spec
	OriginalDevfileMountPathEnvVar = "DEVWORKSPACE_ORIGINAL_DEVFILE"
)

func getWorkspaceMetaEnvVar() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  MetadataMountPathEnvVar,
			Value: metadataMountPath,
		},
		{
			Name:  FlattenedDevfileMountPathEnvVar,
			Value: path.Join(metadataMountPath, flattenedYamlFilename),
		},
		{
			Name:  OriginalDevfileMountPathEnvVar,
			Value: path.Join(metadataMountPath, originalYamlFilename),
		},
	}
}
