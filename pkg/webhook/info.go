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
	"fmt"

	admregv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	webhookCfg "github.com/devfile/devworkspace-operator/webhook/workspace"
)

var webhooksCreationTimestamp = metav1.Time{}

func GetWebhooksCreationTimestamp(client client.Client) (metav1.Time, error) {
	if webhooksCreationTimestamp.IsZero() {
		webhook := &admregv1.MutatingWebhookConfiguration{}
		err := client.Get(context.TODO(), types.NamespacedName{Name: webhookCfg.MutateWebhookCfgName}, webhook)
		if err != nil {
			return metav1.Time{}, fmt.Errorf("failed to get webhook: %w", err)
		}
		webhooksCreationTimestamp = webhook.CreationTimestamp
	}
	return webhooksCreationTimestamp, nil
}
