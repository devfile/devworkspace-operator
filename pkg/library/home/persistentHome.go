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

package home

import (
	"fmt"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	devfilevalidation "github.com/devfile/api/v2/pkg/validation"
	"k8s.io/utils/pointer"

	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

// Returns a modified copy of the given DevWorkspace's Template Spec which contains an additional
// Devfile volume 'persistentHome' that is mounted  to `/home/user/` for every container component defined in the DevWorkspace.
// An error is returned if the addition of the 'persistentHome' volume would result
// in an invalid DevWorkspace.
func AddPersistentHomeVolume(workspace *common.DevWorkspaceWithConfig) (*v1alpha2.DevWorkspaceTemplateSpec, error) {
	dwTemplateSpecCopy := workspace.Spec.Template.DeepCopy()
	homeVolume := v1alpha2.Component{
		Name: constants.HomeVolumeName,
		ComponentUnion: v1alpha2.ComponentUnion{
			Volume: &v1alpha2.VolumeComponent{},
		},
	}
	homeVolumeMount := v1alpha2.VolumeMount{
		Name: constants.HomeVolumeName,
		Path: constants.HomeUserDirectory,
	}

	dwTemplateSpecCopy.Components = append(dwTemplateSpecCopy.Components, homeVolume)
	for _, component := range dwTemplateSpecCopy.Components {
		if component.Container == nil {
			continue
		}
		component.Container.VolumeMounts = append(component.Container.VolumeMounts, homeVolumeMount)
	}

	err := devfilevalidation.ValidateComponents(dwTemplateSpecCopy.Components)
	if err != nil {
		return nil, fmt.Errorf("addition of %s volume would render DevWorkspace invalid: %w", constants.HomeVolumeName, err)
	}

	return dwTemplateSpecCopy, nil
}

// Returns true if `persistUserHome` is enabled in the DevWorkspaceOperatorConfig
// and none of the container components in the DevWorkspace mount a volume to `/home/user/`.
// Returns false otherwise.
func NeedsPersistentHomeDirectory(workspace *common.DevWorkspaceWithConfig) bool {
	if !pointer.BoolDeref(workspace.Config.Workspace.PersistUserHome.Enabled, false) {
		return false
	}
	for _, component := range workspace.Spec.Template.Components {
		if component.Container == nil {
			continue
		}
		for _, volumeMount := range component.Container.VolumeMounts {
			if volumeMount.Path == constants.HomeUserDirectory {
				// If a volume is already being mounted to /home/user/, it takes precedence
				// over the DWO-provisioned home directory volume.
				return false
			}
		}
	}
	return true
}
