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
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateWebhookClusterRoleBinding(client crclient.Client,
	ctx context.Context,
	namespace string) error {

	roleBinding, err := getSpecClusterRoleBinding(namespace)
	if err != nil {
		return err
	}

	if err := client.Create(ctx, roleBinding); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		existingCfg, err := getClusterRoleBinding(ctx, client)
		if err != nil {
			return err
		}
		roleBinding.ResourceVersion = existingCfg.ResourceVersion
		err = client.Update(ctx, roleBinding)
		if err != nil {
			return err
		}
		log.Info("Updated webhook server cluster role binding")
	} else {
		log.Info("Created webhook server cluster role binding")
	}

	return nil
}

func getSpecClusterRoleBinding(namespace string) (*v1.ClusterRoleBinding, error) {
	clusterRoleBinding := &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   server.WebhookServerSAName,
			Labels: server.WebhookServerAppLabels(),
		},
		Subjects: []v1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      server.WebhookServerSAName,
				Namespace: namespace,
			},
		},
		RoleRef: v1.RoleRef{
			Kind:     "ClusterRole",
			Name:     server.WebhookServerSAName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	return clusterRoleBinding, nil
}

func getClusterRoleBinding(ctx context.Context, client crclient.Client) (*v1.ClusterRoleBinding, error) {
	crb := &v1.ClusterRoleBinding{}
	namespacedName := types.NamespacedName{
		Name: server.WebhookServerSAName,
	}
	err := client.Get(ctx, namespacedName, crb)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return crb, nil
}
