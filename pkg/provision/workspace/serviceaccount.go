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

package workspace

import (
	"context"
	"fmt"

	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	securityv1 "github.com/openshift/api/security/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/devfile/devworkspace-operator/pkg/common"
)

func SyncServiceAccount(
	workspace *common.DevWorkspaceWithConfig,
	additionalAnnotations map[string]string,
	clusterAPI sync.ClusterAPI) (serviceAccountName string, err error) {
	saName := common.ServiceAccountName(workspace)

	specSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: workspace.Namespace,
			Labels:    common.ServiceAccountLabels(workspace),
		},
		// note: autoMountServiceAccount := true comes from a hardcoded value in prerequisites.go
		AutomountServiceAccountToken: pointer.Bool(true),
	}

	if len(additionalAnnotations) > 0 {
		specSA.Annotations = map[string]string{}
		for annotKey, annotVal := range additionalAnnotations {
			specSA.Annotations[annotKey] = annotVal
		}
	}

	if workspace.Config.Workspace.ServiceAccount.ServiceAccountName != "" {
		// Add ownerref for the current workspace. The object may have existing owner references
		if err := controllerutil.SetOwnerReference(workspace.DevWorkspace, specSA, clusterAPI.Scheme); err != nil {
			return "", err
		}
	} else {
		// Only add controller reference if shared ServiceAccount is not being used.
		if err := controllerutil.SetControllerReference(workspace.DevWorkspace, specSA, clusterAPI.Scheme); err != nil {
			return "", err
		}
	}

	if _, err := sync.SyncObjectWithCluster(specSA, clusterAPI); err != nil {
		return "", dwerrors.WrapSyncError(err)
	}

	return saName, nil
}

// FinalizeServiceAccount removes the workspace service account from the SCC specified by the controller.devfile.io/scc attribute.
//
// Deprecated: This should no longer be needed as the serviceaccount finalizer is no longer added to workspaces (and workspaces
// do not update SCCs) but is kept here in order to clear finalizers from existing workspaces on deletion.
func FinalizeServiceAccount(workspace *common.DevWorkspaceWithConfig, ctx context.Context, nonCachingClient crclient.Client) (retry bool, err error) {
	saName := common.ServiceAccountName(workspace)
	namespace := workspace.Namespace
	if !workspace.Spec.Template.Attributes.Exists(constants.WorkspaceSCCAttribute) {
		return false, nil
	}
	sccName := workspace.Spec.Template.Attributes.GetString(constants.WorkspaceSCCAttribute, nil)

	return removeSCCFromServiceAccount(saName, namespace, sccName, ctx, nonCachingClient)
}

// Deprecated: This function is left in place ot ensure changes to SCCs can be undone when a workspace is deleted. However,
// the DevWorkspace Operator no longer updates SCCs, so this functionality is not required for new workspaces.
func removeSCCFromServiceAccount(saName, namespace, sccName string, ctx context.Context, nonCachingClient crclient.Client) (retry bool, err error) {
	serviceaccount := fmt.Sprintf("system:serviceaccount:%s:%s", namespace, saName)

	scc := &securityv1.SecurityContextConstraints{}
	if err := nonCachingClient.Get(ctx, types.NamespacedName{Name: sccName}, scc); err != nil {
		switch {
		case k8sErrors.IsForbidden(err):
			return false, fmt.Errorf("operator does not have permissions to get the '%s' SecurityContextConstraints", sccName)
		case k8sErrors.IsNotFound(err):
			return false, fmt.Errorf("requested SecurityContextConstraints '%s' not found on cluster", sccName)
		default:
			return false, err
		}
	}

	found := false
	var newUsers []string
	for _, user := range scc.Users {
		if user == serviceaccount {
			found = true
			continue
		}
		newUsers = append(newUsers, user)
	}
	if !found {
		return false, err
	}

	scc.Users = newUsers

	if err := nonCachingClient.Update(ctx, scc); err != nil {
		switch {
		case k8sErrors.IsForbidden(err):
			return false, fmt.Errorf("operator does not have permissions to update the '%s' SecurityContextConstraints", sccName)
		case k8sErrors.IsConflict(err):
			return true, nil
		default:
			return false, err
		}
	}

	return false, nil
}
