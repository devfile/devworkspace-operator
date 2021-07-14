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
	"github.com/devfile/devworkspace-operator/controllers/workspace/provision"
	"github.com/devfile/devworkspace-operator/pkg/library/container"
)

// The EphemeralStorageProvisioner provisions all workspace storage as emptyDir volumes.
// Any local changes are lost when the workspace is stopped; its lifetime is tied to the
// underlying pod.
type EphemeralStorageProvisioner struct{}

var _ Provisioner = (*EphemeralStorageProvisioner)(nil)

func (e EphemeralStorageProvisioner) NeedsStorage(_ *dw.DevWorkspaceTemplateSpec) bool {
	// Since all volumes are emptyDir, no PVC needs to be provisioned
	return false
}

func (e EphemeralStorageProvisioner) ProvisionStorage(podAdditions *v1alpha1.PodAdditions, workspace *dw.DevWorkspace, _ provision.ClusterAPI) error {
	persistent, ephemeral, projects := getWorkspaceVolumes(workspace)
	if _, err := addEphemeralVolumesToPodAdditions(podAdditions, persistent); err != nil {
		return err
	}
	if _, err := addEphemeralVolumesToPodAdditions(podAdditions, ephemeral); err != nil {
		return err
	}
	if projects != nil {
		if _, err := addEphemeralVolumesToPodAdditions(podAdditions, []dw.Component{*projects}); err != nil {
			return err
		}
	} else {
		if container.AnyMountSources(workspace.Spec.Template.Components) {
			projectsComponent := dw.Component{Name: "projects"}
			projectsComponent.Volume = &dw.VolumeComponent{}
			if _, err := addEphemeralVolumesToPodAdditions(podAdditions, []dw.Component{projectsComponent}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e EphemeralStorageProvisioner) CleanupWorkspaceStorage(_ *dw.DevWorkspace, _ provision.ClusterAPI) error {
	return nil
}
