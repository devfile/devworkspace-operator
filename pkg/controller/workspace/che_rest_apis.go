//
// Copyright (c) 2019-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package workspace

import (
	"github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	"github.com/che-incubator/che-workspace-operator/pkg/config"
	corev1 "k8s.io/api/core/v1"
)

const cheRestAPIsName = "che-rest-apis"
const cheRestApisPort = 9999

func getCheRestApisComponent(workspaceName, workspaceId, namespace string) v1alpha1.ComponentDescription {
	container := corev1.Container{
		Image:           config.ControllerCfg.GetCheRestApisDockerImage(),
		ImagePullPolicy: corev1.PullPolicy(config.ControllerCfg.GetSidecarPullPolicy()),
		Name:            cheRestAPIsName,
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: int32(cheRestApisPort),
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env: []corev1.EnvVar{
			{
				Name:  "CHE_WORKSPACE_NAME",
				Value: workspaceName,
			},
			{
				Name:  "CHE_WORKSPACE_ID",
				Value: workspaceId,
			},
			{
				Name:  "CHE_WORKSPACE_NAMESPACE",
				Value: namespace,
			},
		},
	}

	return v1alpha1.ComponentDescription{
		Name: cheRestAPIsName,
		PodAdditions: v1alpha1.PodAdditions{
			Containers: []corev1.Container{container},
		},
		ComponentMetadata: v1alpha1.ComponentMetadata{
			Containers: map[string]v1alpha1.ContainerDescription{
				cheRestAPIsName: {
					Attributes: map[string]string{
						config.RestApisContainerSourceAttribute: config.RestApisRecipeSourceToolAttribute,
					},
					Ports: []int{cheRestApisPort},
				},
			},
			Endpoints: []v1alpha1.Endpoint{
				{
					Attributes: map[v1alpha1.EndpointAttribute]string{
						v1alpha1.PUBLIC_ENDPOINT_ATTRIBUTE: "false",
						//v1alpha1.TYPE_ENDPOINT_ATTRIBUTE: "ide",
						v1alpha1.PROTOCOL_ENDPOINT_ATTRIBUTE: "tcp",
					},
					Name: cheRestAPIsName,
					Port: int64(cheRestApisPort),
				},
			},
		},
	}
}
