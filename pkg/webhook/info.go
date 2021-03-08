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

package webhook

import (
	"context"

	webhookCfg "github.com/devfile/devworkspace-operator/webhook/workspace"
	"k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var webhooksCreationTimestamp = metav1.Time{}

// GetMutatingWebhook returns the mutating webhook used by the controller, or a kubernetes error if an
// error is encountered while retrieving webhooks. The returned error can be checked via IsNotFound()
func GetMutatingWebhook(client client.Client) (*v1beta1.MutatingWebhookConfiguration, error) {
	webhook := &v1beta1.MutatingWebhookConfiguration{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: webhookCfg.MutateWebhookCfgName}, webhook)
	return webhook, err
}

// GetValidatingWebhook returns the validating webhook used by the controller, or a kubernetes error if an
// error is encountered while retrieving webhooks. The returned error can be checked via IsNotFound()
func GetValidatingWebhook(client client.Client) (*v1beta1.ValidatingWebhookConfiguration, error) {
	webhook := &v1beta1.ValidatingWebhookConfiguration{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: webhookCfg.ValidateWebhookCfgName}, webhook)
	return webhook, err
}
