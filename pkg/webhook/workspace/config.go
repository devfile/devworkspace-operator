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
package workspace

import (
	"context"
	"github.com/che-incubator/che-workspace-operator/internal/controller"
	"github.com/che-incubator/che-workspace-operator/pkg/controller/ownerref"
	"github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/config"
	"github.com/che-incubator/che-workspace-operator/pkg/webhook/workspace/handler"
	"k8s.io/api/admissionregistration/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

//SetUp sets up mutate/validating webhooks that provides exec access into workspace for creator only
func SetUp(webhookServer *webhook.Server, ctx context.Context) error {
	if config.ControllerCfg.GetWebhooksEnabled() == "false" {
		log.Info("Webhooks are disabled")
		return nil
	}

	log.Info("Configuring workspace webhooks")
	c, err := controller.CreateClient()
	if err != nil {
		return err
	}

	mutateWebhookCfg := buildMutateWebhookCfg()

	ownRef, err := ownerref.FindControllerOwner(ctx, c)
	if err != nil {
		return err
	}

	//TODO we need to watch owned webhook configuration and clean up old ones

	//TODO For some reasons it's still possible to update reference by user
	//TODO Investigate if we can block it. The same issue is valid for Deployment owner
	mutateWebhookCfg.SetOwnerReferences([]metav1.OwnerReference{*ownRef})

	if err := c.Create(ctx, mutateWebhookCfg); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		// Webhook Configuration already exists, we want to update it
		// as we do not know if any fields might have changed.
		existingCfg := &v1beta1.MutatingWebhookConfiguration{}
		err := c.Get(ctx, types.NamespacedName{
			Name:      mutateWebhookCfg.Name,
			Namespace: mutateWebhookCfg.Namespace,
		}, existingCfg)

		mutateWebhookCfg.ResourceVersion = existingCfg.ResourceVersion
		err = c.Update(ctx, mutateWebhookCfg)
		if err != nil {
			return err
		}
		log.Info("Updated workspace mutating webhook configuration")
	} else {
		log.Info("Created workspace mutating webhook configuration")
	}

	webhookServer.Register(mutateWebhookPath, &webhook.Admission{Handler: &handler.WorkspaceResourcesMutator{}})

	validateWebhookCfg := buildValidatingWebhookCfg()
	validateWebhookCfg.SetOwnerReferences([]metav1.OwnerReference{*ownRef})

	if err := c.Create(ctx, validateWebhookCfg); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		// Webhook Configuration already exists, we want to update it
		// as we do not know if any fields might have changed.
		existingCfg := &v1beta1.ValidatingWebhookConfiguration{}
		err := c.Get(ctx, types.NamespacedName{
			Name:      validateWebhookCfg.Name,
			Namespace: validateWebhookCfg.Namespace,
		}, existingCfg)

		validateWebhookCfg.ResourceVersion = existingCfg.ResourceVersion
		err = c.Update(ctx, validateWebhookCfg)
		if err != nil {
			return err
		}
		log.Info("Updated workspace validating webhook configuration")
	} else {
		log.Info("Created workspace validating webhook configuration")
	}

	webhookServer.Register(validateWebhookPath, &webhook.Admission{Handler: &handler.WorkspaceValidator{}})

	return nil
}
