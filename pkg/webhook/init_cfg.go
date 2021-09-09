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
	"k8s.io/apimachinery/pkg/types"

	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/devfile/devworkspace-operator/webhook/workspace"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// WebhookCfgsInit initializes the webhook that denies everything until webhook server is started successfully
func WebhookCfgsInit(client crclient.Client, ctx context.Context, namespace string) error {
	configuration := workspace.BuildMutateWebhookCfg(namespace)

	err := client.Create(ctx, configuration, &crclient.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			clusterCfg := &admregv1.MutatingWebhookConfiguration{}
			err := client.Get(ctx, types.NamespacedName{Namespace: namespace}, clusterCfg)
			if err != nil {
				return err
			}
			if len(clusterCfg.Webhooks) == 0 {
				log.Info(fmt.Sprintf("Init mutating webhooks configuration %s already exists", configuration.Name))
				return nil
			}

			webhookServerNamespace := clusterCfg.Webhooks[0].ClientConfig.Service.Namespace
			if webhookServerNamespace != namespace {
				// TODO Handle more advanced logic to check if controller deployment exists in namespace webhookServerNamespace
				// TODO So, it will make it possible newer DWO take control over existing webhooks if the previous is uninstalled but webhooks are not cleaned up
				panic(fmt.Sprintf("Webhooks already exist and point to %s. Probably operator in already installed " +
					"in that namespace. Failing to prevent conflict and endless CRs reconciling", webhookServerNamespace))
			}

			log.Info(fmt.Sprintf("Mutating webhooks configuration %s already exists", configuration.Name))
			return nil
		} else {
			return err
		}
	}
	log.Info(fmt.Sprintf("Created webhooks configuration %s", configuration.Name))
	return nil
}
