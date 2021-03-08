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

package controllers

import (
	"fmt"

	"github.com/devfile/devworkspace-operator/pkg/webhook"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"

	devworkspace "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/config"
)

// validateRestrictedWorkspace checks that a devworkspace was created after workspace-related mutating webhooks
// and ensures a creator ID label is applied to the workspace.
//
// If error is not nil, a user-readable message is returned that can be propagated to the user to explain the issue.
func (r *DevWorkspaceReconciler) validateRestrictedWorkspace(workspace *devworkspace.DevWorkspace) (msg string, err error) {
	mutatingWebhook, err := webhook.GetMutatingWebhook(r.Client)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return "Restricted access workspace require webhooks to be installed, but they are not found on the cluster. " +
					"Contact an administrator to fix Operator installation.",
				fmt.Errorf("failed to read mutating webhook configuration: %w", err)
		}
		return "Failed to read webhooks on cluster.", fmt.Errorf("failed to read mutating webhook configuration: %w", err)
	}
	if workspace.CreationTimestamp.Before(&mutatingWebhook.CreationTimestamp) {
		return "DevWorkspace was created before current webhooks were installed and must be recreated to successfully start",
			fmt.Errorf("devworkspace created before webhooks")
	}

	validatingWebhook, err := webhook.GetValidatingWebhook(r.Client)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return "Restricted access workspace require webhooks to be installed, but they are not found on the cluster. " +
					"Contact an administrator to fix Operator installation.",
				fmt.Errorf("failed to read validating webhook configuration: %w", err)
		}
		return "Failed to read webhooks on cluster.", fmt.Errorf("failed to read validating webhook configuration: %w", err)
	}
	if workspace.CreationTimestamp.Before(&validatingWebhook.CreationTimestamp) {
		return "DevWorkspace was created before current webhooks were installed and must be recreated to successfully start",
			fmt.Errorf("devworkspace created before webhooks")
	}

	if _, present := workspace.Labels[config.WorkspaceCreatorLabel]; !present {
		return "DevWorkspace was created without creator ID label. It must be recreated to resolve the issue",
			fmt.Errorf("devworkspace does not have creator label applied")
	}

	return "", nil
}
