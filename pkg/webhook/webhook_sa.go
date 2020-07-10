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

	"github.com/devfile/devworkspace-operator/webhook/server"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateWebhookSA(client crclient.Client,
	ctx context.Context,
	namespace string) error {

	serviceAccount, err := getSpecServiceAccount(namespace)
	if err != nil {
		return err
	}

	if err := client.Create(ctx, serviceAccount); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		existingCfg, err := getClusterServiceAccount(client, ctx, namespace)
		if err != nil {
			return err
		}
		serviceAccount.ResourceVersion = existingCfg.ResourceVersion
		err = client.Update(ctx, serviceAccount)
		if err != nil {
			return err
		}
		log.Info("Updated webhook server service account")
	} else {
		log.Info("Created webhook server service account")
	}

	return nil
}

func getSpecServiceAccount(namespace string) (*corev1.ServiceAccount, error) {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.WebhookServerSAName,
			Namespace: namespace,
			Labels:    server.WebhookServerAppLabels(),
		},
	}

	return serviceAccount, nil
}

func getClusterServiceAccount(client crclient.Client, ctx context.Context, namespace string) (*corev1.ServiceAccount, error) {
	serviceAccount := &corev1.ServiceAccount{}
	namespacedName := types.NamespacedName{
		Name:      server.WebhookServerSAName,
		Namespace: namespace,
	}
	err := client.Get(ctx, namespacedName, serviceAccount)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return serviceAccount, nil
}
