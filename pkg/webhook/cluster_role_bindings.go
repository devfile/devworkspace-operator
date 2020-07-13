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
