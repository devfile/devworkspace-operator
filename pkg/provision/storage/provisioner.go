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

package storage

import (
	"errors"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

// UnsupportedStorageStrategy is used when the controller is configured with an invalid storage strategy
var UnsupportedStorageStrategy = errors.New("configured storage type not supported")

// Provisioner is an interface for rewriting volumeMounts in a pod according to a storage policy (e.g. common PVC for all mounts, etc.)
type Provisioner interface {
	// ProvisionStorage rewrites the volumes and volumeMounts in podAdditions to match the current storage policy and syncs any
	// out-of-pod required objects to the cluster.
	// Returns NotReadyError to signify that storage is not ready, ProvisioningError when a fatal issue is encountered,
	// and other error if there is an unexpected problem.
	ProvisionStorage(podAdditions *v1alpha1.PodAdditions, workspace *common.DevWorkspaceWithConfig, clusterAPI sync.ClusterAPI) error
	// NeedsStorage returns whether the current workspace needs a PVC to be provisioned, given this storage strategy.
	NeedsStorage(workspace *dw.DevWorkspaceTemplateSpec) bool
	// CleanupWorkspaceStorage removes any objects provisioned by in the ProvisionStorage step that aren't automatically removed when a
	// DevWorkspace is deleted (e.g. delete subfolders in a common PVC assigned to the workspace)
	// Returns nil on success (DevWorkspace can be deleted), NotReadyError if additional reconciles are necessary, ProvisioningError when
	// a fatal issue is encountered, and any other error if an unexpected problem arises.
	CleanupWorkspaceStorage(workspace *common.DevWorkspaceWithConfig, clusterAPI sync.ClusterAPI) error
}

// GetProvisioner returns the storage provisioner that should be used for the current workspace
func GetProvisioner(workspace *common.DevWorkspaceWithConfig) (Provisioner, error) {
	storageClass := workspace.Spec.Template.Attributes.GetString(constants.DevWorkspaceStorageTypeAttribute, nil)
	if storageClass == "" {
		return &CommonStorageProvisioner{}, nil
	}
	switch storageClass {
	case constants.CommonStorageClassType:
		return &CommonStorageProvisioner{}, nil
	case constants.PerWorkspaceStorageClassType:
		return &PerWorkspaceStorageProvisioner{}, nil
	case constants.AsyncStorageClassType:
		return &AsyncStorageProvisioner{}, nil
	case constants.EphemeralStorageClassType:
		return &EphemeralStorageProvisioner{}, nil
	case constants.PerUserStorageClassType:
		return &CommonStorageProvisioner{}, nil
	default:
		return nil, UnsupportedStorageStrategy
	}
}
