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

	"github.com/devfile/devworkspace-operator/pkg/config"

	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"k8s.io/client-go/rest"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("webhook")

func SetupWebhooks(ctx context.Context, cfg *rest.Config) error {
	if config.ControllerCfg.GetWebhooksEnabled() == "false" {
		log.Info("Webhooks are disabled. Skipping deploying webhook server")
		return nil
	}

	namespace, err := k8sutil.GetOperatorNamespace()
	if err != nil {
		return err
	}

	client, err := crclient.New(cfg, crclient.Options{})
	if err != nil {
		return fmt.Errorf("failed to create new client: %w", err)
	}
	// Set up the certs
	log.Info("Setting up the init webhooks configurations")
	err = WebhookCfgsInit(client, ctx, namespace)
	if err != nil {
		return err
	}

	// Set up the service account
	log.Info("Setting up the service account")
	err = CreateWebhookSA(client, ctx, namespace)
	if err != nil {
		return err
	}

	// Set up the cluster roles
	log.Info("Setting up the cluster roles")
	err = CreateWebhookRole(client, ctx)
	if err != nil {
		return err
	}

	// Set up the cluster role binding
	log.Info("Setting up the cluster role binding")
	err = CreateWebhookClusterRoleBinding(client, ctx, namespace)
	if err != nil {
		return err
	}

	// Set up the certs
	log.Info("Setting up the secure certs")
	err = SetupWebhookCerts(client, ctx, namespace)
	if err != nil {
		return err
	}

	// Set up the deployment
	log.Info("Creating the webhook server deployment")
	err = CreateWebhookServerDeployment(client, ctx, namespace)
	if err != nil {
		return err
	}

	return nil
}
