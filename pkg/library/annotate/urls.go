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
