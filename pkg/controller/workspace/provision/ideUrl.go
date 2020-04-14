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

package provision

import (
	"context"
	"github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
)

func SyncWorkspaceIdeURL(workspace *v1alpha1.Workspace, exposedEndpoints map[string][]v1alpha1.ExposedEndpoint, clusterAPI ClusterAPI) ProvisioningStatus {
	ideUrl := getIdeUrl(exposedEndpoints)

	if workspace.Status.IdeUrl == ideUrl {
		return ProvisioningStatus{
			Continue: true,
		}
	}
	workspace.Status.IdeUrl = ideUrl
	err := clusterAPI.Client.Status().Update(context.TODO(), workspace)
	return ProvisioningStatus{
		Continue: false,
		Requeue:  true,
		Err:      err,
	}
}

func getIdeUrl(exposedEndpoints map[string][]v1alpha1.ExposedEndpoint) string {
	for _, endpoints := range exposedEndpoints {
		for _, endpoint := range endpoints {
			if endpoint.Attributes[v1alpha1.TYPE_ENDPOINT_ATTRIBUTE] == "ide" {
				return endpoint.Url
			}
		}
	}
	return ""
}
