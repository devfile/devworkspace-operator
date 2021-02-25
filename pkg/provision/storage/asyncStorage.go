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
	"errors"
	"fmt"
	"time"

	"github.com/devfile/devworkspace-operator/pkg/library/constants"
	corev1 "k8s.io/api/core/v1"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/workspace/provision"
	"github.com/devfile/devworkspace-operator/pkg/provision/storage/asyncstorage"
)

// The AsyncStorageProvisioner provisions one PVC per namespace and creates an ssh deployment that syncs data into that PVC.
// Workspaces are provisioned with sync sidecars that sync data from the workspace to the async ssh deployment. All storage
// attached to a workspace is emptyDir volumes.
type AsyncStorageProvisioner struct{}

var _ Provisioner = (*AsyncStorageProvisioner)(nil)

func (*AsyncStorageProvisioner) NeedsStorage(workspace *dw.DevWorkspaceTemplateSpec) bool {
	return needsStorage(workspace)
}

func (p *AsyncStorageProvisioner) ProvisionStorage(podAdditions *v1alpha1.PodAdditions, workspace *dw.DevWorkspace, clusterAPI provision.ClusterAPI) error {
	// Add ephemeral volumes
	_, ephemeralVolumes, projectsVolume := getWorkspaceVolumes(workspace)
	_, err := addEphemeralVolumesToPodAdditions(podAdditions, ephemeralVolumes)
	if err != nil {
		return &ProvisioningError{Message: "Failed to add ephemeral volumes to workspace", Err: err}
	}
	if projectsVolume != nil && projectsVolume.Volume.Ephemeral {
		if _, err := addEphemeralVolumesToPodAdditions(podAdditions, []dw.Component{*projectsVolume}); err != nil {
			return &ProvisioningError{Message: "Failed to add projects volume to workspace", Err: err}
		}
	}

	// If persistent storage is not needed, we're done
	if !p.NeedsStorage(&workspace.Spec.Template) {
		return nil
	}

	// Sync SSH keypair to cluster
	secret, configmap, err := asyncstorage.GetOrCreateSSHConfig(workspace, clusterAPI)
	if err != nil {
		if errors.Is(err, asyncstorage.NotReadyError) {
			return &NotReadyError{
				Message:      fmt.Sprintf("setting up configuration for async storage"),
				RequeueAfter: 1 * time.Second,
			}
		}
		return err
	}

	// Create common PVC if needed
	clusterPVC, err := syncCommonPVC(workspace.Namespace, clusterAPI)
	if err != nil {
		return err
	}

	// Create async server deployment
	deploy, err := asyncstorage.SyncWorkspaceSyncDeploymentToCluster(workspace.Namespace, configmap, clusterPVC, clusterAPI)
	if err != nil {
		if errors.Is(err, asyncstorage.NotReadyError) {
			return &NotReadyError{
				Message:      "waiting for async storage server deployment to be ready",
				RequeueAfter: 1 * time.Second,
			}
		}
		return err
	}

	// Create service for async storage server
	_, err = asyncstorage.SyncWorkspaceSyncServiceToCluster(deploy, clusterAPI)
	if err != nil {
		if errors.Is(err, asyncstorage.NotReadyError) {
			return &NotReadyError{
				Message:      "waiting for async storage service to be ready",
				RequeueAfter: 1 * time.Second,
			}
		}
		return err
	}

	volumes, err := p.addVolumesForAsyncStorage(podAdditions, workspace)
	if err != nil {
		return err
	}

	sshSecretVolume := asyncstorage.GetVolumeFromSecret(secret)
	asyncSidecar := asyncstorage.GetAsyncSidecar(sshSecretVolume.Name, volumes)
	podAdditions.Containers = append(podAdditions.Containers, *asyncSidecar)
	podAdditions.Volumes = append(podAdditions.Volumes, *sshSecretVolume)

	return nil
}

func (*AsyncStorageProvisioner) CleanupWorkspaceStorage(workspace *dw.DevWorkspace, clusterAPI provision.ClusterAPI) error {
	return nil
}

func (*AsyncStorageProvisioner) addVolumesForAsyncStorage(podAdditions *v1alpha1.PodAdditions, workspace *dw.DevWorkspace) (volumes []corev1.Volume, err error) {
	persistentVolumes, _, _ := getWorkspaceVolumes(workspace)

	addedVolumes, err := addEphemeralVolumesToPodAdditions(podAdditions, persistentVolumes)
	if err != nil {
		return nil, err
	}
	volumes = append(volumes, addedVolumes...)

	projectsVolume, needed := processProjectsVolume(&workspace.Spec.Template)
	if needed {
		if projectsVolume != nil && !projectsVolume.Volume.Ephemeral {
			vol, err := addEphemeralVolumesToPodAdditions(podAdditions, []dw.Component{*projectsVolume})
			if err != nil {
				return nil, err
			}
			volumes = append(volumes, vol...)
		} else {
			vol := corev1.Volume{
				Name: constants.ProjectsVolumeName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}
			podAdditions.Volumes = append(podAdditions.Volumes, vol)
			volumes = append(volumes, vol)
		}
	}

	return volumes, nil
}
