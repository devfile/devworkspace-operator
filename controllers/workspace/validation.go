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

	"k8s.io/api/admissionregistration/v1beta1"

	"github.com/devfile/devworkspace-operator/pkg/webhook"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

type controllerWebhooks struct {
	mutating   *v1beta1.MutatingWebhookConfiguration
	validating *v1beta1.ValidatingWebhookConfiguration
}

// validateWebhooksConfig validates that expected webhooks are present on the cluster. If an error is encountered or webhooks
// do not exist, a user-facing message and error are returned; otherwise, the webhook specs are returned, msg is empty, and err
// is nil.
func (r *DevWorkspaceReconciler) validateWebhooksConfig() (webhooks *controllerWebhooks, msg string, err error) {
	mutatingWebhook, err := webhook.GetMutatingWebhook(r.Client)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, "Operator requires webhooks to be installed, but they are not found on the cluster. " +
					"Contact an administrator to fix Operator installation.",
				fmt.Errorf("failed to read mutating webhook configuration: %w", err)
		}
		return nil, "Failed to read webhooks on cluster.", fmt.Errorf("failed to read mutating webhook configuration: %w", err)
	}
	validatingWebhook, err := webhook.GetValidatingWebhook(r.Client)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, "Operator requires webhooks to be installed, but they are not found on the cluster. " +
					"Contact an administrator to fix Operator installation.",
				fmt.Errorf("failed to read validating webhook configuration: %w", err)
		}
		return nil, "Failed to read webhooks on cluster.", fmt.Errorf("failed to read validating webhook configuration: %w", err)
	}
	webhooks = &controllerWebhooks{
		mutating:   mutatingWebhook,
		validating: validatingWebhook,
	}
	return webhooks, "", nil
}
