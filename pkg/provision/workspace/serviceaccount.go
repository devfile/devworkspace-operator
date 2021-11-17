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
	"fmt"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	securityv1 "github.com/openshift/api/security/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

	if workspace.Spec.Template.Attributes.Exists(constants.WorkspaceSCCAttribute) {
		sccName := workspace.Spec.Template.Attributes.GetString(constants.WorkspaceSCCAttribute, nil)
		retry, err := addSCCToServiceAccount(specSA.Name, specSA.Namespace, sccName, clusterAPI)
		if err != nil {
			return ServiceAcctProvisioningStatus{ProvisioningStatus: ProvisioningStatus{FailStartup: true, Err: err}}
		}
		if retry {
			return ServiceAcctProvisioningStatus{ProvisioningStatus: ProvisioningStatus{Requeue: true}}
		}
	}

	return ServiceAcctProvisioningStatus{
		ProvisioningStatus: ProvisioningStatus{
			Continue: true,
		},
		ServiceAccountName: saName,
	}
}

func addSCCToServiceAccount(saName, namespace, sccName string, clusterAPI sync.ClusterAPI) (retry bool, err error) {
	serviceaccount := fmt.Sprintf("system:serviceaccount:%s:%s", namespace, saName)

	// TODO: Check if we can access the SCC

	scc := &securityv1.SecurityContextConstraints{}
	if err := clusterAPI.Client.Get(clusterAPI.Ctx, types.NamespacedName{Name: sccName}, scc); err != nil {
		switch {
		case k8sErrors.IsUnauthorized(err):
			return false, fmt.Errorf("operator does not have permissions to get the '%s' SecurityContextConstraint", sccName)
		case k8sErrors.IsNotFound(err):
			return false, fmt.Errorf("requested SCC '%s' not found on cluster", sccName)
		default:
			return false, err
		}
	}

	for _, user := range scc.Users {
		if user == serviceaccount {
			// This serviceaccount is already added to the SCC
			return false, nil
		}
	}

	scc.Users = append(scc.Users, serviceaccount)
	if err := clusterAPI.Client.Update(clusterAPI.Ctx, scc); err != nil {
		switch {
		case k8sErrors.IsUnauthorized(err):
			return false, fmt.Errorf("operator does not have permissions to update the '%s' SecurityContextConstraint", sccName)
		case k8sErrors.IsConflict(err):
			return true, nil
		default:
			return false, err
		}
	}

	return false, nil
}
