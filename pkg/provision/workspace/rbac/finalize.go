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

package rbac

import (
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
)

func FinalizeRBAC(workspace *common.DevWorkspaceWithConfig, api sync.ClusterAPI) error {
	if workspace.Spec.Template.Attributes.Exists(constants.WorkspaceSCCAttribute) {
		if err := finalizeSCCRBAC(workspace, api); err != nil {
			return err
		}
	}
	saName := common.ServiceAccountName(workspace)
	roleName := common.WorkspaceRoleName()
	rolebindingName := common.WorkspaceRolebindingName()
	numWorkspaces, err := countNonDeletedWorkspaces(workspace.Namespace, api)
	if err != nil {
		return err
	}
	if numWorkspaces == 0 {
		if err := deleteRole(roleName, workspace.Namespace, api); err != nil {
			return err
		}
		if err := deleteRolebinding(rolebindingName, workspace.Namespace, api); err != nil {
			return err
		}
		return nil
	}
	if err := removeServiceAccountFromRolebinding(saName, workspace.Namespace, rolebindingName, api); err != nil {
		return err
	}
	return nil
}

func finalizeSCCRBAC(workspace *common.DevWorkspaceWithConfig, api sync.ClusterAPI) error {
	sccName := workspace.Spec.Template.Attributes.GetString(constants.WorkspaceSCCAttribute, nil)
	saName := common.ServiceAccountName(workspace)
	roleName := common.WorkspaceSCCRoleName(sccName)
	rolebindingName := common.WorkspaceSCCRolebindingName(sccName)
	numWorkspaces, err := countNonDeletedWorkspacesUsingSCC(sccName, workspace.Namespace, api)
	if err != nil {
		return err
	}
	if numWorkspaces == 0 {
		if err := deleteRole(roleName, workspace.Namespace, api); err != nil {
			return err
		}
		if err := deleteRolebinding(rolebindingName, workspace.Namespace, api); err != nil {
			return err
		}
		return nil
	}
	if err := removeServiceAccountFromRolebinding(saName, workspace.Namespace, rolebindingName, api); err != nil {
		return err
	}
	return nil
}

func countNonDeletedWorkspaces(namespace string, api sync.ClusterAPI) (int, error) {
	count := 0
	allWorkspaces := &dw.DevWorkspaceList{}
	allWorkspacesListOptions := &client.ListOptions{
		Namespace: namespace,
	}
	if err := api.Client.List(api.Ctx, allWorkspaces, allWorkspacesListOptions); err != nil {
		return -1, err
	}
	for _, workspace := range allWorkspaces.Items {
		if workspace.DeletionTimestamp != nil {
			// ignore workspaces that are being deleted
			continue
		}
		count = count + 1
	}
	return count, nil
}

func countNonDeletedWorkspacesUsingSCC(sccName, namespace string, api sync.ClusterAPI) (int, error) {
	count := 0
	allWorkspaces := &dw.DevWorkspaceList{}
	allWorkspacesListOptions := &client.ListOptions{
		Namespace: namespace,
	}
	if err := api.Client.List(api.Ctx, allWorkspaces, allWorkspacesListOptions); err != nil {
		return -1, err
	}
	for _, workspace := range allWorkspaces.Items {
		if workspace.DeletionTimestamp != nil {
			// ignore workspaces that are being deleted
			continue
		}
		attrs := workspace.Spec.Template.Attributes
		if attrs.Exists(constants.WorkspaceSCCAttribute) && attrs.GetString(constants.WorkspaceSCCAttribute, nil) == sccName {
			count = count + 1
		}
	}
	return count, nil
}
