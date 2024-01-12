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

package controllers

import (
	"fmt"

	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/webhook"
)

// validateCreatorLabel checks that a devworkspace was created after workspace-related mutating webhooks
// and ensures a creator ID label is applied to the workspace. If webhooks are disabled, validation succeeds by
// default.
//
// If error is not nil, a user-readable message is returned that can be propagated to the user to explain the issue.
func (r *DevWorkspaceReconciler) validateCreatorLabel(workspace *common.DevWorkspaceWithConfig) (msg string, err error) {
	if _, present := workspace.Labels[constants.DevWorkspaceCreatorLabel]; !present {
		return "DevWorkspace was created without creator ID label. It must be recreated to resolve the issue",
			fmt.Errorf("devworkspace does not have creator label applied")
	}

	webhooksTimestamp, err := webhook.GetWebhooksCreationTimestamp(r.Client)
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
