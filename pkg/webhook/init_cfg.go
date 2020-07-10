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

package webhook

import (
	"context"
	"fmt"

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
			log.Info(fmt.Sprintf("Mutating webhooks configuration %s already exists", configuration.Name))
			return nil
		} else {
			return err
		}
	}
	log.Info(fmt.Sprintf("Created webhooks configuration %s", configuration.Name))
	return nil
}
