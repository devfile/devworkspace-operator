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
	"errors"
	"fmt"
	"os"

	webhook_k8s "github.com/devfile/devworkspace-operator/pkg/webhook/kubernetes"
	webhook_openshift "github.com/devfile/devworkspace-operator/pkg/webhook/openshift"

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
		if errors.Is(err, k8sutil.ErrRunLocal) {
			// local mode. Just read watch namespace env var set by operator sdk
			namespace = os.Getenv("WATCH_NAMESPACE")
		} else {
			return err
		}
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

	err = setUpWebhookServerRBAC(ctx, err, client, namespace)
	if err != nil {
		return err
	}

	if config.ControllerCfg.IsOpenShift() {
		// Set up the certs for OpenShift
		log.Info("Setting up the OpenShift webhook server secure service")
		err := webhook_openshift.SetupSecureService(client, ctx, namespace)
		if err != nil {
			return err
		}
	} else {
		// Set up the certs for kubernetes
		log.Info("Setting up the Kubernetes webhook server secure service")
		err := webhook_k8s.SetupSecureService(client, ctx, namespace)
		if err != nil {
			return err
		}
		log.Info("Warning: the webhook server cert in use is not suitable for production. If you want to use this in production please set up certs with a certificate manager")
	}

	// Set up the deployment
	log.Info("Creating the webhook server deployment")
	err = CreateWebhookServerDeployment(client, ctx, namespace)
	if err != nil {
		return err
	}

	return nil
}

// setUpWebhookServerRBAC sets required service account, cluster role, and cluster role binding
// for creating a webhook server
func setUpWebhookServerRBAC(ctx context.Context, err error, client crclient.Client, namespace string) error {
	// Set up the service account
	log.Info("Setting up the webhook server service account")
	err = CreateWebhookSA(client, ctx, namespace)
	if err != nil {
		return err
	}

	// Set up the cluster roles
	log.Info("Setting up the webhook server cluster roles")
	err = CreateWebhookClusterRole(client, ctx)
	if err != nil {
		return err
	}

	// Set up the cluster role binding
	log.Info("Setting up the webhook server cluster role binding")
	err = CreateWebhookClusterRoleBinding(client, ctx, namespace)
	if err != nil {
		return err
	}
	return nil
}
