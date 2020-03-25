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
	"github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (h *WebhookHandler) mutateMetadataOnCreate(o *metav1.ObjectMeta) error {
	if o.GetAnnotations() == nil {
		return fmt.Errorf("annotations are missing while %s is required", model.WorkspaceCreatorAnnotation)
	} else {
		if _, ok := o.GetAnnotations()[model.WorkspaceCreatorAnnotation]; !ok {
			return fmt.Errorf("'%s' annotation is missing", model.WorkspaceCreatorAnnotation)
		}
	}

	if o.GetLabels() == nil {
		return fmt.Errorf("labels are missing while %s is required", model.WorkspaceIDLabel)
	} else {
		if _, ok := o.GetLabels()[model.WorkspaceIDLabel]; !ok {
			return fmt.Errorf("'%s' label is missing", model.WorkspaceIDLabel)
		}
	}

	return nil
}

func (h *WebhookHandler) mutateMetadataOnUpdate(oldMeta, newMeta *metav1.ObjectMeta) (bool, error) {
	if oldMeta.Annotations == nil {
		oldMeta.Annotations = map[string]string{}
	}
	if newMeta.Annotations == nil {
		newMeta.Annotations = map[string]string{}
	}
	updatedAnnotations, err := mutateAnnotationsOnUpdate(oldMeta.Annotations, newMeta.Annotations)
	if err != nil {
		return false, err
	}

	if oldMeta.Labels == nil {
		oldMeta.Labels = map[string]string{}
	}
	if newMeta.Labels == nil {
		newMeta.Labels = map[string]string{}
	}
	updatedLabels, err := mutateLabelsOnUpdate(oldMeta.Labels, newMeta.Labels)
	if err != nil {
		return false, err
	}

	return updatedAnnotations || updatedLabels, nil
}

func mutateAnnotationsOnUpdate(old, new map[string]string) (bool, error) {
	oldCreator, found := old[model.WorkspaceCreatorAnnotation]
	if !found {
		return false, fmt.Errorf("'%s' annotation is required. Update Controller and restart your workspace", model.WorkspaceCreatorAnnotation)
	}

	newCreator, found := new[model.WorkspaceCreatorAnnotation]
	if !found {
		new[model.WorkspaceCreatorAnnotation] = oldCreator
		return true, nil
	}

	if newCreator != oldCreator {
		return false, fmt.Errorf("annotation '%s' is assigned once workspace is created and is immutable", model.WorkspaceCreatorAnnotation)
	}

	return false, nil
}

func mutateLabelsOnUpdate(old map[string]string, new map[string]string) (bool, error) {
	oldWorkpaceId, found := old[model.WorkspaceIDLabel]
	if !found {
		return false, fmt.Errorf("'%s' label is required. Update Controller and restart your workspace", model.WorkspaceIDLabel)
	}

	newCreator, found := new[model.WorkspaceIDLabel]
	if !found {
		new[model.WorkspaceIDLabel] = oldWorkpaceId
		return true, nil
	}

	if newCreator != oldWorkpaceId {
		return false, fmt.Errorf("'%s' label is assigned once workspace is created and is immutable", model.WorkspaceIDLabel)
	}

	return false, nil
}
