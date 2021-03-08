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
package handler

import (
	"fmt"

	"github.com/devfile/devworkspace-operator/pkg/constants"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (h *WebhookHandler) mutateMetadataOnCreate(o *metav1.ObjectMeta) error {
	if _, ok := o.Labels[constants.WorkspaceIDLabel]; !ok {
		return fmt.Errorf("'%s' label is missing", constants.WorkspaceIDLabel)
	}

	if _, ok := o.Labels[constants.WorkspaceCreatorLabel]; !ok {
		return fmt.Errorf("'%s' label is missing", constants.WorkspaceCreatorLabel)
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
	oldWorkpaceId, found := old[constants.WorkspaceIDLabel]
	if !found {
		return false, fmt.Errorf("'%s' label is required. Update Controller and restart your workspace", constants.WorkspaceIDLabel)
	}

	newCreator, found := new[constants.WorkspaceIDLabel]
	if !found {
		new[constants.WorkspaceIDLabel] = oldWorkpaceId
		return true, nil
	}

	if newCreator != oldWorkpaceId {
		return false, fmt.Errorf("'%s' label is assigned once workspace is created and is immutable", constants.WorkspaceIDLabel)
	}
	return false, nil
}

func mutateCreatorLabel(old map[string]string, new map[string]string) (bool, error) {
	oldCreator, found := old[constants.WorkspaceCreatorLabel]
	if !found {
		return false, fmt.Errorf("'%s' label is required. Update Controller and restart your workspace", constants.WorkspaceCreatorLabel)
	}

	newCreator, found := new[constants.WorkspaceCreatorLabel]
	if !found {
		new[constants.WorkspaceCreatorLabel] = oldCreator
		return true, nil
	}

	if newCreator != oldCreator {
		return false, fmt.Errorf("label '%s' is assigned once workspace is created and is immutable", constants.WorkspaceCreatorLabel)
	}

	return false, nil
}
