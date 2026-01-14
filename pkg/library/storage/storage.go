//
// Copyright (c) 2019-2025 Red Hat, Inc.
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

package storage

import (
	"context"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/provision/storage"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetWorkspacePVCInfo determines the PVC name and workspace path based on the storage provisioner used.
// This function can be used by both backup and restore operations to ensure consistent PVC resolution logic.
//
// Returns:
//   - pvcName: The name of the PVC that stores workspace data
//   - workspacePath: The path within the PVC where workspace data is stored
//   - error: Any error that occurred during PVC resolution
func GetWorkspacePVCInfo(
	ctx context.Context,
	workspace *dw.DevWorkspace,
	config *controllerv1alpha1.OperatorConfiguration,
	k8sClient client.Client,
	log logr.Logger,
) (pvcName string, workspacePath string, err error) {
	workspaceWithConfig := &common.DevWorkspaceWithConfig{}
	workspaceWithConfig.DevWorkspace = workspace
	workspaceWithConfig.Config = config

	storageProvisioner, err := storage.GetProvisioner(workspaceWithConfig)
	if err != nil {
		return "", "", err
	}
	if !storageProvisioner.NeedsStorage(&workspace.Spec.Template) {
		// No storage provisioned for this workspace
		return "", "", nil
	}

	if _, ok := storageProvisioner.(*storage.PerWorkspaceStorageProvisioner); ok {
		pvcName := common.PerWorkspacePVCName(workspace.Status.DevWorkspaceId)
		return pvcName, constants.DefaultProjectsSourcesRoot, nil

	} else if _, ok := storageProvisioner.(*storage.CommonStorageProvisioner); ok {
		pvcName := constants.DefaultWorkspacePVCName
		if config.Workspace != nil && config.Workspace.PVCName != "" {
			pvcName = config.Workspace.PVCName
		}
		return pvcName, workspace.Status.DevWorkspaceId + constants.DefaultProjectsSourcesRoot, nil
	}
	return "", "", nil
}
