//
// Copyright (c) 2019-2022 Red Hat, Inc.
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
	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/library/container"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
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

func (e EphemeralStorageProvisioner) ProvisionStorage(podAdditions *v1alpha1.PodAdditions, workspaceWithConfig *common.DevWorkspaceWithConfig, _ sync.ClusterAPI) error {
	persistent, ephemeral, projects := getWorkspaceVolumes(&workspaceWithConfig.DevWorkspace)
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
		if container.AnyMountSources(workspaceWithConfig.Spec.Template.Components) {
			projectsComponent := dw.Component{Name: "projects"}
			projectsComponent.Volume = &dw.VolumeComponent{}
			if _, err := addEphemeralVolumesToPodAdditions(podAdditions, []dw.Component{projectsComponent}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e EphemeralStorageProvisioner) CleanupWorkspaceStorage(_ *common.DevWorkspaceWithConfig, _ sync.ClusterAPI) error {
	return nil
}
