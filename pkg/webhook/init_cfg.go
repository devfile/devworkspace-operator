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

	admv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/devfile/devworkspace-operator/webhook/workspace"
)

// WebhookCfgsInit initializes the webhook that denies everything until webhook server is started successfully
func WebhookCfgsInit(client crclient.Client, ctx context.Context, namespace string) error {
	configuration := workspace.BuildMutateWebhookCfg(namespace)

	err := client.Create(ctx, configuration, &crclient.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			log.Info(fmt.Sprintf("Mutating webhooks configuration %s already exists", configuration.Name))
			return checkExistingConfigForConflict(client, ctx, namespace)
		} else {
			return err
		}
	}
	log.Info(fmt.Sprintf("Created webhooks configuration %s", configuration.Name))
	return nil
}

func checkExistingConfigForConflict(client crclient.Client, ctx context.Context, serviceNamespace string) error {
	existingCfg := &admv1.MutatingWebhookConfiguration{}
	err := client.Get(ctx, types.NamespacedName{Name: workspace.MutateWebhookCfgName}, existingCfg)
	if err != nil {
		return err
	}
	for _, webhook := range existingCfg.Webhooks {
		if webhook.ClientConfig.Service.Namespace != serviceNamespace {
			return fmt.Errorf("conflicting webhook definition found on cluster, webhook %s clientConfig points at namespace %s", webhook.Name, webhook.ClientConfig.Service.Namespace)
		}
	}
	return nil
}
