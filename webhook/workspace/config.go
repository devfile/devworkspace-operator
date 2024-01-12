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

package workspace

import (
	"context"
	"errors"
	"fmt"

	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/devfile/devworkspace-operator/webhook/server"

	admregv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientConfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// Configure configures mutate/validating webhooks that provides exec access into workspace for creator only
func Configure(ctx context.Context) error {
	log.Info("Configuring devworkspace webhooks")
	c, err := createClient()
	if err != nil {
		return err
	}

	namespace, err := infrastructure.GetOperatorNamespace()
	if err != nil {
		return err
	}

	mutateWebhookCfg := BuildMutateWebhookCfg(namespace)
	validateWebhookCfg := buildValidatingWebhookCfg(namespace)

	if !server.IsSetUp() {
		_, mutatingWebhookErr := getMutatingWebhook(ctx, c, mutateWebhookCfg)
		_, validatingWebhookErr := getValidateWebhook(ctx, c, validateWebhookCfg)

		// No errors from either means that they are on the cluster
		if mutatingWebhookErr == nil || validatingWebhookErr == nil {
			return errors.New(`Webhooks have previously been set up and cannot be removed automatically. Configure the controller to use webhooks again or make sure that all workspaces are stopped, webhooks configuration are removed and then restart the controller`)
		}
		return nil
	}

	saUID, saName, err := controllerSAUID(ctx, c)
	if err != nil {
		return err
	}

	if err := c.Create(ctx, mutateWebhookCfg); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		// Webhook Configuration already exists, we want to update it
		// as we do not know if any fields might have changed.
		existingCfg, err := getMutatingWebhook(ctx, c, mutateWebhookCfg)
		if err != nil {
			return err
		}

		mutateWebhookCfg.ResourceVersion = existingCfg.ResourceVersion
		err = c.Update(ctx, mutateWebhookCfg)
		if err != nil {
			return err
		}
		log.Info("Updated devworkspace mutating webhook configuration")
	} else {
		log.Info("Created devworkspace mutating webhook configuration")
	}

	server.GetWebhookServer().Register(mutateWebhookPath, &webhook.Admission{Handler: NewResourcesMutator(saUID, saName)})

	if err := c.Create(ctx, validateWebhookCfg); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		// Webhook Configuration already exists, we want to update it
		// as we do not know if any fields might have changed.
		existingCfg, err := getValidateWebhook(ctx, c, validateWebhookCfg)
		if err != nil {
			return err
		}

		validateWebhookCfg.ResourceVersion = existingCfg.ResourceVersion
		err = c.Update(ctx, validateWebhookCfg)
		if err != nil {
			return err
		}
		log.Info("Updated devworkspace validating webhook configuration")
	} else {
		log.Info("Created devworkspace validating webhook configuration")
	}

	server.GetWebhookServer().Register(validateWebhookPath, &webhook.Admission{Handler: NewResourcesValidator(saUID, saName)})

	return nil
}

func getMutatingWebhook(ctx context.Context, c client.Client, mutatingWebhookCfg *admregv1.MutatingWebhookConfiguration) (*admregv1.MutatingWebhookConfiguration, error) {
	existingCfg := &admregv1.MutatingWebhookConfiguration{}
	err := c.Get(ctx, types.NamespacedName{
		Name:      mutatingWebhookCfg.Name,
		Namespace: mutatingWebhookCfg.Namespace,
	}, existingCfg)
	return existingCfg, err
}

func getValidateWebhook(ctx context.Context, c client.Client, validateWebhookCfg *admregv1.ValidatingWebhookConfiguration) (*admregv1.ValidatingWebhookConfiguration, error) {
	existingCfg := &admregv1.ValidatingWebhookConfiguration{}
	err := c.Get(ctx, types.NamespacedName{
		Name:      validateWebhookCfg.Name,
		Namespace: validateWebhookCfg.Namespace,
	}, existingCfg)
	return existingCfg, err
}

func controllerSAUID(ctx context.Context, c client.Client) (string, string, error) {
	saName, err := config.GetWorkspaceControllerSA()
	if err != nil {
		return "", "", err
	}
	namespace, err := infrastructure.GetOperatorNamespace()
	if err != nil {
		return "", "", err
	}
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      saName,
	}
	sa := &corev1.ServiceAccount{}
	err = c.Get(ctx, namespacedName, sa)
	if err != nil {
		return "", "", err
	}
	fullSAName := fmt.Sprintf("system:serviceaccount:%s:%s", namespace, saName)
	return string(sa.UID), fullSAName, nil
}

// createClient creates Controller client with default config
// or returns error if any happens
func createClient() (client.Client, error) {
	cfg, err := clientConfig.GetConfig()
	if err != nil {
		return nil, err
	}

	c, err := client.New(cfg, client.Options{})
	if err != nil {
		return nil, err
	}

	return c, nil
}
