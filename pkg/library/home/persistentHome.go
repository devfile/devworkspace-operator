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

package home

import (
	"fmt"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	devfilevalidation "github.com/devfile/api/v2/pkg/validation"
	"github.com/devfile/devworkspace-operator/pkg/provision/storage"
	"k8s.io/utils/pointer"

	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

const initScript = `(echo "Checking for stow command"
STOW_COMPLETE=/home/user/.stow_completed
if command -v stow &> /dev/null; then
  if  [ ! -f $STOW_COMPLETE ]; then
    echo "Running stow command"
    stow . -t /home/user/ -d /home/tooling/ --no-folding -v 2 > /home/user/.stow.log 2>&1
    cp -n /home/tooling/.viminfo /home/user/.viminfo
    cp -n /home/tooling/.bashrc /home/user/.bashrc
    cp -n /home/tooling/.bash_profile /home/user/.bash_profile
    touch $STOW_COMPLETE
  else
    echo "Stow command already run. If you wish to re-run it, delete $STOW_COMPLETE from the persistent volume and restart the workspace."
  fi
else
  echo "Stow command not found"
fi) || true
`

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

	// Add default init container only if not disabled and no custom init is configured
	if workspace.Config.Workspace.PersistUserHome.DisableInitContainer == nil || !*workspace.Config.Workspace.PersistUserHome.DisableInitContainer {
		err := addInitContainer(dwTemplateSpecCopy)
		if err != nil {
			return nil, fmt.Errorf("failed to add init container for home persistence setup: %w", err)
		}
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

// Returns true if the following criteria is met:
// - `persistUserHome` is enabled in the DevWorkspaceOperatorConfig
// - The storage strategy used by the DevWorkspace supports home persistence
// - None of the container components in the DevWorkspace mount a volume to `/home/user/`.
// - Persistent storage is required for the DevWorkspace
// Returns false otherwise.
func NeedsPersistentHomeDirectory(workspace *common.DevWorkspaceWithConfig) bool {
	if !PersistUserHomeEnabled(workspace) || !storageStrategySupportsPersistentHome(workspace) {
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
	return storage.WorkspaceNeedsStorage(&workspace.Spec.Template)
}

func PersistUserHomeEnabled(workspace *common.DevWorkspaceWithConfig) bool {
	return pointer.BoolDeref(workspace.Config.Workspace.PersistUserHome.Enabled, false)
}

// Returns true if the workspace's storage strategy supports persisting the user home directory.
// The storage strategies which support home persistence are: per-user/common, per-workspace & async.
// The ephemeral storage strategy does not support home persistence.
func storageStrategySupportsPersistentHome(workspace *common.DevWorkspaceWithConfig) bool {
	storageClass := workspace.Spec.Template.Attributes.GetString(constants.DevWorkspaceStorageTypeAttribute, nil)
	return storageClass != constants.EphemeralStorageClassType
}

func addInitContainer(dwTemplateSpec *v1alpha2.DevWorkspaceTemplateSpec) error {

	if initComponentExists(dwTemplateSpec) {
		return fmt.Errorf("component named %s already exists in the devworkspace", constants.HomeInitComponentName)
	}

	if initCommandExists(dwTemplateSpec) {
		return fmt.Errorf("command with id %s already exists in the devworkspace", constants.HomeInitEventId)
	}

	if initEventExists(dwTemplateSpec) {
		return fmt.Errorf("event with id %s already exists in the devworkspace", constants.HomeInitEventId)
	}

	initContainer := inferInitContainer(dwTemplateSpec)
	if initContainer == nil {
		return fmt.Errorf("cannot infer initcontainer for home persistence setup")
	}

	addInitContainerComponent(dwTemplateSpec, initContainer)

	if dwTemplateSpec.Commands == nil {
		dwTemplateSpec.Commands = []v1alpha2.Command{}
	}

	if dwTemplateSpec.Events == nil {
		dwTemplateSpec.Events = &v1alpha2.Events{}
	}

	dwTemplateSpec.Commands = append(dwTemplateSpec.Commands, v1alpha2.Command{
		Id: constants.HomeInitEventId,
		CommandUnion: v1alpha2.CommandUnion{
			Apply: &v1alpha2.ApplyCommand{
				Component: constants.HomeInitComponentName,
			},
		},
	})

	dwTemplateSpec.Events.PreStart = append(dwTemplateSpec.Events.PreStart, constants.HomeInitEventId)

	return nil
}

func initComponentExists(dwTemplateSpec *v1alpha2.DevWorkspaceTemplateSpec) bool {
	if dwTemplateSpec.Components == nil {
		return false
	}
	for _, component := range dwTemplateSpec.Components {
		if component.Name == constants.HomeInitComponentName {
			return true
		}
	}
	return false

}

func initCommandExists(dwTemplateSpec *v1alpha2.DevWorkspaceTemplateSpec) bool {
	if dwTemplateSpec.Commands == nil {
		return false
	}
	for _, command := range dwTemplateSpec.Commands {
		if command.Id == constants.HomeInitEventId {
			return true
		}
	}
	return false
}

func initEventExists(dwTemplateSpec *v1alpha2.DevWorkspaceTemplateSpec) bool {
	if dwTemplateSpec.Events == nil {
		return false
	}
	for _, event := range dwTemplateSpec.Events.PreStart {
		if event == constants.HomeInitEventId {
			return true
		}
	}
	return false

}

func addInitContainerComponent(dwTemplateSpec *v1alpha2.DevWorkspaceTemplateSpec, initContainer *v1alpha2.Container) {
	initComponent := v1alpha2.Component{
		Name: constants.HomeInitComponentName,
		ComponentUnion: v1alpha2.ComponentUnion{
			Container: &v1alpha2.ContainerComponent{
				Container: *initContainer,
			},
		},
	}
	dwTemplateSpec.Components = append(dwTemplateSpec.Components, initComponent)
}

func inferInitContainer(dwTemplateSpec *v1alpha2.DevWorkspaceTemplateSpec) *v1alpha2.Container {
	image := InferWorkspaceImage(dwTemplateSpec)
	if image != "" {
		return &v1alpha2.Container{
			Image:   image,
			Command: []string{"/bin/sh", "-c"},
			Args:    []string{initScript},
		}
	}
	return nil
}
