//
// Copyright (c) 2019-2021 Red Hat, Inc.
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

package workspace

import (
	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/devfile/devworkspace-operator/pkg/common"
)

type ServiceAcctProvisioningStatus struct {
	ProvisioningStatus
	ServiceAccountName string
}

func SyncServiceAccount(
	workspace *dw.DevWorkspace,
	additionalAnnotations map[string]string,
	clusterAPI sync.ClusterAPI) ServiceAcctProvisioningStatus {
	// note: autoMountServiceAccount := true comes from a hardcoded value in prerequisites.go
	autoMountServiceAccount := true
	saName := common.ServiceAccountName(workspace.Status.DevWorkspaceId)

	specSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: workspace.Namespace,
			Labels: map[string]string{
				constants.DevWorkspaceIDLabel:   workspace.Status.DevWorkspaceId,
				constants.DevWorkspaceNameLabel: workspace.Name,
			},
		},
		AutomountServiceAccountToken: &autoMountServiceAccount,
	}

	if len(additionalAnnotations) > 0 {
		specSA.Annotations = map[string]string{}
		for annotKey, annotVal := range additionalAnnotations {
			specSA.Annotations[annotKey] = annotVal
		}
	}

	err := controllerutil.SetControllerReference(workspace, specSA, clusterAPI.Scheme)
	if err != nil {
		return ServiceAcctProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{
				Err: err,
			},
		}
	}

	_, err = sync.SyncObjectWithCluster(specSA, clusterAPI)
	switch t := err.(type) {
	case nil:
		break
	case *sync.NotInSyncError:
		return ServiceAcctProvisioningStatus{ProvisioningStatus: ProvisioningStatus{Requeue: true}}
	case *sync.UnrecoverableSyncError:
		return ServiceAcctProvisioningStatus{ProvisioningStatus: ProvisioningStatus{FailStartup: true, Err: t.Cause}}
	default:
		return ServiceAcctProvisioningStatus{ProvisioningStatus: ProvisioningStatus{Err: err}}
	}

	return ServiceAcctProvisioningStatus{
		ProvisioningStatus: ProvisioningStatus{
			Continue: true,
		},
		ServiceAccountName: saName,
	}
}
