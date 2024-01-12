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

package webhook

import (
	"context"
	"fmt"
	"os"

	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	webhook_k8s "github.com/devfile/devworkspace-operator/pkg/webhook/kubernetes"
	webhook_openshift "github.com/devfile/devworkspace-operator/pkg/webhook/openshift"

	"k8s.io/client-go/rest"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("webhook")

func SetupWebhooks(ctx context.Context, cfg *rest.Config) error {
	namespace, err := infrastructure.GetOperatorNamespace()
	if err != nil {
		namespace = os.Getenv(infrastructure.WatchNamespaceEnvVar)
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

	secretName, err := config.GetWebhooksSecretName()
	if err != nil {
		return fmt.Errorf("could not deploy webhooks server: %w", err)
	}
	if infrastructure.IsOpenShift() {
		// Set up the certs for OpenShift
		log.Info("Setting up the OpenShift webhook server secure service")
		log.Info("Injecting serving cert using the Service CA operator")
		err := webhook_openshift.SetupSecureService(client, ctx, secretName, namespace)
		if err != nil {
			return err
		}
	} else {
		log.Info("Setting up the Kubernetes webhook server secure service")
		log.Info(fmt.Sprintf("Using certificate stored in secret '%s' to serve webhooks", secretName))
		err = webhook_k8s.SetupSecureService(client, ctx, namespace)
		if err != nil {
			return err
		}
	}

	// Set up the deployment
	log.Info("Creating the webhook server deployment")
	err = CreateWebhookServerDeployment(client, ctx, secretName, namespace)
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
