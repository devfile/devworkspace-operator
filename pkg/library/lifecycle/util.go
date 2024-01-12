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

package lifecycle

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

func getContainerWithName(name string, containers []corev1.Container) (*corev1.Container, error) {
	for idx, container := range containers {
		if container.Name == name {
			return &containers[idx], nil
		}
	}
	return nil, fmt.Errorf("container component with name %s not found", name)
}
