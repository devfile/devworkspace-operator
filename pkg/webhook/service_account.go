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

	"github.com/devfile/devworkspace-operator/webhook/server"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
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
		if needsUpdate(serviceAccount, existingCfg) {
			serviceAccount.ResourceVersion = existingCfg.ResourceVersion
			err = client.Update(ctx, serviceAccount)
			if err != nil {
				return err
			}
			log.Info("Updated webhook server service account")
		} else {
			log.Info("Webhook server service account up to date")
		}
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

func needsUpdate(spec, cluster *corev1.ServiceAccount) bool {
	return !equality.Semantic.DeepDerivative(spec.Labels, cluster.Labels)
}
