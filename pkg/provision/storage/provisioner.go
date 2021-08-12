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

package storage

import (
	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	wsprovision "github.com/devfile/devworkspace-operator/pkg/provision/workspace"
)

// Provisioner is an interface for rewriting volumeMounts in a pod according to a storage policy (e.g. common PVC for all mounts, etc.)
type Provisioner interface {
	// ProvisionStorage rewrites the volumes and volumeMounts in podAdditions to match the current storage policy and syncs any
	// out-of-pod required objects to the cluster.
	// Returns NotReadyError to signify that storage is not ready, ProvisioningError when a fatal issue is encountered,
	// and other error if there is an unexpected problem.
	ProvisionStorage(podAdditions *v1alpha1.PodAdditions, workspace *dw.DevWorkspace, clusterAPI wsprovision.ClusterAPI) error
	// NeedsStorage returns whether the current workspace needs a PVC to be provisioned, given this storage strategy.
	NeedsStorage(workspace *dw.DevWorkspaceTemplateSpec) bool
	// CleanupWorkspaceStorage removes any objects provisioned by in the ProvisionStorage step that aren't automatically removed when a
	// DevWorkspace is deleted (e.g. delete subfolders in a common PVC assigned to the workspace)
	// Returns nil on success (DevWorkspace can be deleted), NotReadyError if additional reconciles are necessary, ProvisioningError when
	// a fatal issue is encountered, and any other error if an unexpected problem arises.
	CleanupWorkspaceStorage(workspace *dw.DevWorkspace, clusterAPI wsprovision.ClusterAPI) error
}

// GetProvisioner returns the storage provisioner that should be used for the current workspace
func GetProvisioner(workspace *dw.DevWorkspace) (Provisioner, error) {
	// TODO: Figure out what to do if a workspace changes the storage type after its been created
	// e.g. common -> async so as to not leave files on PVCs after removal. Maybe block changes to
	// this label via webhook?
	storageClass := workspace.Spec.Template.Attributes.GetString(constants.DevWorkspaceStorageTypeAtrr, nil)
	if storageClass == "" {
		return &CommonStorageProvisioner{}, nil
	}
	switch storageClass {
	case constants.CommonStorageClassType:
		return &CommonStorageProvisioner{}, nil
	case constants.AsyncStorageClassType:
		return &AsyncStorageProvisioner{}, nil
	case constants.EphemeralStorageClassType:
		return &EphemeralStorageProvisioner{}, nil
	default:
		return nil, UnsupportedStorageStrategy
	}
}
