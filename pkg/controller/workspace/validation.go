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

package workspace

import (
	"fmt"

	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/webhook"
	devworkspace "github.com/devfile/kubernetes-api/pkg/apis/workspaces/v1alpha1"
)

func (r *ReconcileWorkspace) validateCreatorTimestamp(workspace *devworkspace.DevWorkspace) error {
	if config.ControllerCfg.GetWebhooksEnabled() != "true" {
		return nil
	}
	if _, present := workspace.Labels[config.WorkspaceCreatorLabel]; !present {
		return fmt.Errorf("devworkspace does not have creator label -- it must be recreated to resolve the issue")
	}

	webhooksTimestamp, err := webhook.GetWebhooksCreationTimestamp(r.client)
	if err != nil {
		return fmt.Errorf("webhooks not set up yet: %w", err)
	}
	if workspace.CreationTimestamp.Before(&webhooksTimestamp) {
		return fmt.Errorf("devworkspace created before webhooks -- it must be recreated to resolve the issue")
	}

	return nil
}
