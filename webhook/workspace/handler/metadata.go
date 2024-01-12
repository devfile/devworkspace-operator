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

package handler

import (
	"fmt"

	"github.com/devfile/devworkspace-operator/pkg/constants"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (h *WebhookHandler) mutateMetadataOnCreate(o *metav1.ObjectMeta) error {
	devworkspaceId, ok := o.Labels[constants.DevWorkspaceIDLabel]
	if !ok {
		return fmt.Errorf("'%s' label is missing", constants.DevWorkspaceIDLabel)
	}

	// An empty devworkspaceId is used for resources that are associated with multiple workspaces
	// e.g. the async storage server deployment.
	if _, ok := o.Labels[constants.DevWorkspaceCreatorLabel]; !ok && devworkspaceId != "" {
		return fmt.Errorf("'%s' label is missing", constants.DevWorkspaceCreatorLabel)
	}

	return nil
}

func (h *WebhookHandler) mutateMetadataOnUpdate(oldMeta, newMeta *metav1.ObjectMeta) (bool, error) {
	if newMeta.Labels == nil {
		newMeta.Labels = map[string]string{}
	}
	updatedLabels, err := mutateLabelsOnUpdate(oldMeta.Labels, newMeta.Labels)
	if err != nil {
		return false, err
	}

	return updatedLabels, nil
}

func mutateLabelsOnUpdate(old map[string]string, new map[string]string) (bool, error) {
	modifiedWorkspaceId, err := mutateWorkspaceIdLabel(old, new)
	if err != nil {
		return false, err
	}

	modifiedCreator, err := mutateCreatorLabel(old, new)

	if err != nil {
		return false, err
	}

	return modifiedWorkspaceId || modifiedCreator, nil
}

func mutateWorkspaceIdLabel(old map[string]string, new map[string]string) (bool, error) {
	// There are cases where the old version of a resource does not have a DevWorkspaceIDLabel set
	// and we need to enable it to be added after the fact (e.g. old instances of the async storage
	// server before the devworkspaceId label was added)
	oldWorkpaceId, oldFound := old[constants.DevWorkspaceIDLabel]
	newWorkspaceId, newFound := new[constants.DevWorkspaceIDLabel]
	switch {
	case !newFound && !oldFound:
		return false, fmt.Errorf("'%s' label is required. Update Controller and restart your DevWorkspace", constants.DevWorkspaceIDLabel)
	case !newFound && oldFound:
		new[constants.DevWorkspaceIDLabel] = oldWorkpaceId
		return true, nil
	case newFound && !oldFound:
		return false, nil
	default: // oldFound && newFound
		if newWorkspaceId != oldWorkpaceId {
			return false, fmt.Errorf("the '%s' label is assigned when a devworkspace is created and is immutable", constants.DevWorkspaceIDLabel)
		}
		return false, nil
	}
}

func mutateCreatorLabel(old map[string]string, new map[string]string) (bool, error) {
	devworkspaceId := old[constants.DevWorkspaceIDLabel]
	oldCreator, found := old[constants.DevWorkspaceCreatorLabel]
	// Empty devworkspaceID is used for common resources (e.g. async storage server deployment) which does not get a creator ID
	if !found && devworkspaceId != "" {
		return false, fmt.Errorf("'%s' label is required. Update Controller and restart your DevWorkspace", constants.DevWorkspaceCreatorLabel)
	}

	newCreator, found := new[constants.DevWorkspaceCreatorLabel]
	if !found && devworkspaceId != "" {
		new[constants.DevWorkspaceCreatorLabel] = oldCreator
		return true, nil
	}

	if newCreator != oldCreator {
		return false, fmt.Errorf("label '%s' is assigned once devworkspace is created and is immutable", constants.DevWorkspaceCreatorLabel)
	}

	return false, nil
}
