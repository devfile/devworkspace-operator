//
// Copyright (c) 2019-2021 Red Hat, Inc.
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
	if _, ok := o.Labels[constants.DevWorkspaceIDLabel]; !ok {
		return fmt.Errorf("'%s' label is missing", constants.DevWorkspaceIDLabel)
	}

	if _, ok := o.Labels[constants.DevWorkspaceCreatorLabel]; !ok {
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
	oldWorkpaceId, found := old[constants.DevWorkspaceIDLabel]
	if !found {
		return false, fmt.Errorf("'%s' label is required. Update Controller and restart your DevWorkspace", constants.DevWorkspaceIDLabel)
	}

	newCreator, found := new[constants.DevWorkspaceIDLabel]
	if !found {
		new[constants.DevWorkspaceIDLabel] = oldWorkpaceId
		return true, nil
	}

	if newCreator != oldWorkpaceId {
		return false, fmt.Errorf("'%s' label is assigned once devworkspace is created and is immutable", constants.DevWorkspaceIDLabel)
	}
	return false, nil
}

func mutateCreatorLabel(old map[string]string, new map[string]string) (bool, error) {
	oldCreator, found := old[constants.DevWorkspaceCreatorLabel]
	if !found {
		return false, fmt.Errorf("'%s' label is required. Update Controller and restart your DevWorkspace", constants.DevWorkspaceCreatorLabel)
	}

	newCreator, found := new[constants.DevWorkspaceCreatorLabel]
	if !found {
		new[constants.DevWorkspaceCreatorLabel] = oldCreator
		return true, nil
	}

	if newCreator != oldCreator {
		return false, fmt.Errorf("label '%s' is assigned once devworkspace is created and is immutable", constants.DevWorkspaceCreatorLabel)
	}

	return false, nil
}
