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

package annotate

import (
	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

func AddURLAttributesToEndpoints(workspace *dw.DevWorkspaceTemplateSpec, exposedEndpoints map[string]v1alpha1.ExposedEndpointList) {
	for _, component := range workspace.Components {
		if component.Container == nil {
			continue
		}
		container := component.Container
		endpoints := exposedEndpoints[component.Name]
		for _, exposedEndpoint := range endpoints {
			if containerEndpoint := getContainerEndpointByName(exposedEndpoint.Name, container); containerEndpoint != nil {
				if containerEndpoint.Attributes == nil {
					containerEndpoint.Attributes = attributes.Attributes{}
				}
				containerEndpoint.Attributes.PutString(constants.EndpointURLAttribute, exposedEndpoint.Url)
			}
		}
	}
}

func getContainerEndpointByName(name string, container *dw.ContainerComponent) *dw.Endpoint {
	for idx, endpoint := range container.Endpoints {
		if endpoint.Name == name {
			return &container.Endpoints[idx]
		}
	}
	return nil
}
