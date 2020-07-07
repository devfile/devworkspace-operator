//
// Copyright (c) 2019-2020 Red Hat, Inc.
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

	"github.com/devfile/devworkspace-operator/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (h *WebhookHandler) mutateMetadataOnCreate(o *metav1.ObjectMeta) error {
	if _, ok := o.Labels[config.WorkspaceIDLabel]; !ok {
		return fmt.Errorf("'%s' label is missing", config.WorkspaceIDLabel)
	}

	if _, ok := o.Labels[config.WorkspaceCreatorLabel]; !ok {
		return fmt.Errorf("'%s' label is missing", config.WorkspaceCreatorLabel)
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
	oldWorkpaceId, found := old[config.WorkspaceIDLabel]
	if !found {
		return false, fmt.Errorf("'%s' label is required. Update Controller and restart your workspace", config.WorkspaceIDLabel)
	}

	newCreator, found := new[config.WorkspaceIDLabel]
	if !found {
		new[config.WorkspaceIDLabel] = oldWorkpaceId
		return true, nil
	}

	if newCreator != oldWorkpaceId {
		return false, fmt.Errorf("'%s' label is assigned once workspace is created and is immutable", config.WorkspaceIDLabel)
	}
	return false, nil
}

func mutateCreatorLabel(old map[string]string, new map[string]string) (bool, error) {
	oldCreator, found := old[config.WorkspaceCreatorLabel]
	if !found {
		return false, fmt.Errorf("'%s' label is required. Update Controller and restart your workspace", config.WorkspaceCreatorLabel)
	}

	newCreator, found := new[config.WorkspaceCreatorLabel]
	if !found {
		new[config.WorkspaceCreatorLabel] = oldCreator
		return true, nil
	}

	if newCreator != oldCreator {
		return false, fmt.Errorf("label '%s' is assigned once workspace is created and is immutable", config.WorkspaceCreatorLabel)
	}

	return false, nil
}
