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
	"errors"

	"fmt"
	"os"

	"github.com/devfile/devworkspace-operator/pkg/config"
	corev1 "k8s.io/api/core/v1"

	appsv1 "k8s.io/api/apps/v1"

	"github.com/devfile/devworkspace-operator/internal/controller"
	"github.com/devfile/devworkspace-operator/pkg/webhook/server"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"k8s.io/api/admissionregistration/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

const certGenDeploymentName = "che-workspace-controller-cert-gen"

//Configure configures mutate/validating webhooks that provides exec access into workspace for creator only
func Configure(ctx context.Context) error {
	log.Info("Configuring workspace webhooks")
	c, err := controller.CreateClient()
	if err != nil {
		return err
	}

	namespace, err := k8sutil.GetOperatorNamespace()
	if err != nil {
		return err
	}

	// At this point we no longer need the cert generation deployment
	err = deleteCertGenerationDeployment(ctx, c, namespace)
	if err != nil {
		return err
	}

	mutateWebhookCfg := buildMutateWebhookCfg(namespace)
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
		existingCfg, err := getMutatingWebhook(ctx, c, mutateWebhookCfg)
		if err != nil {
			return err
		}

		mutateWebhookCfg.ResourceVersion = existingCfg.ResourceVersion
		err = c.Update(ctx, mutateWebhookCfg)
		if err != nil {
			return err
		}
		log.Info("Updated workspace mutating webhook configuration")
	} else {
		log.Info("Created workspace mutating webhook configuration")
	}

	server.GetWebhookServer().Register(mutateWebhookPath, &webhook.Admission{Handler: NewResourcesMutator(saUID, saName)})

	validateWebhookCfg.SetOwnerReferences([]metav1.OwnerReference{*ownRef})

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
		log.Info("Updated workspace validating webhook configuration")
	} else {
		log.Info("Created workspace validating webhook configuration")
	}

	server.GetWebhookServer().Register(validateWebhookPath, &webhook.Admission{Handler: NewResourcesValidator(saUID, saName)})

	return nil
}

func getMutatingWebhook(ctx context.Context, c client.Client, mutatingWebhookCfg *v1beta1.MutatingWebhookConfiguration) (*v1beta1.MutatingWebhookConfiguration, error) {
	existingCfg := &v1beta1.MutatingWebhookConfiguration{}
	err := c.Get(ctx, types.NamespacedName{
		Name:      mutatingWebhookCfg.Name,
		Namespace: mutatingWebhookCfg.Namespace,
	}, existingCfg)
	return existingCfg, err
}

func getValidateWebhook(ctx context.Context, c client.Client, validateWebhookCfg *v1beta1.ValidatingWebhookConfiguration) (*v1beta1.ValidatingWebhookConfiguration, error) {
	existingCfg := &v1beta1.ValidatingWebhookConfiguration{}
	err := c.Get(ctx, types.NamespacedName{
		Name:      validateWebhookCfg.Name,
		Namespace: validateWebhookCfg.Namespace,
	}, existingCfg)
	return existingCfg, err
}

func controllerSAUID(ctx context.Context, c client.Client) (string, string, error) {
	saName := os.Getenv(config.ControllerServiceAccountNameEnvVar)
	if saName == "" {
		return "", "", fmt.Errorf("could not get service account name")
	}
	namespace, err := k8sutil.GetOperatorNamespace()
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

func deleteCertGenerationDeployment(ctx context.Context, client crclient.Client, namespace string) error {
	if err := client.Delete(ctx, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      certGenDeploymentName,
			Namespace: namespace,
		}}); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}
