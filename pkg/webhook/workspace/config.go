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
	"github.com/che-incubator/che-workspace-operator/pkg/webhook/server"
	"k8s.io/api/admissionregistration/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

//Configure configures mutate/validating webhooks that provides exec access into workspace for creator only
func Configure(ctx context.Context) error {
	log.Info("Configuring workspace webhooks")
	c, err := controller.CreateClient()
	if err != nil {
		return err
	}

	if !server.IsSetUp() {
		log.Info("Webhooks server is not set up. Cleaning up webhook configurations")

		if err := c.Delete(ctx, &v1beta1.MutatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name: mutateWebhookCfgName,
			}}); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
		}
		if err = c.Delete(ctx, &v1beta1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name: validateWebhookCfgName,
			}}); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
		}

		log.Info("Existing workspace related webhook configurations are removed")
		return nil
	}

	mutateWebhookCfg := buildMutateWebhookCfg()

	ownRef, err := controller.FindControllerOwner(ctx, c)
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

	server.GetWebhookServer().Register(mutateWebhookPath, &webhook.Admission{Handler: NewResourcesMutator()})

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

	server.GetWebhookServer().Register(validateWebhookPath, &webhook.Admission{Handler: NewResourcesValidator()})

	return nil
}
