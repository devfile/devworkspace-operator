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
//

package controllers

import (
	"fmt"

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/webhook"
)

// validateCreatorTimestamp checks that a devworkspace was created after workspace-related mutating webhooks
// and ensures a creator ID label is applied to the workspace. If webhooks are disabled, validation succeeds by
// default.
//
// If error is not nil, a user-readable message is returned that can be propagated to the user to explain the issue.
func (r *DevWorkspaceReconciler) validateCreatorTimestamp(workspace *devworkspace.DevWorkspace) (msg string, err error) {
	if config.ControllerCfg.GetWebhooksEnabled() != "true" {
		return "", nil
	}
	if _, present := workspace.Labels[config.WorkspaceCreatorLabel]; !present {
		return "DevWorkspace was created without creator ID label. It must be recreated to resolve the issue",
			fmt.Errorf("devworkspace does not have creator label applied")
	}

	webhooksTimestamp, err := webhook.GetWebhooksCreationTimestamp(r.client)
	if err != nil {
		return "Could not read devworkspace webhooks on cluster. Contact an administrator " +
				"to check logs and fix Operator installation.",
			fmt.Errorf("failed getting webhooks creation timestamp: %w", err)
	}
	if workspace.CreationTimestamp.Before(&webhooksTimestamp) {
		return "DevWorkspace was created before current webhooks were installed and must be recreated to successfully start",
			fmt.Errorf("devworkspace created before webhooks")
	}

	return "", nil
}
