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

func CreateWebhookRole(client crclient.Client,
	ctx context.Context,
	saName string) error {

	clusterRole, err := getSpecRoleBinding(saName)
	if err != nil {
		return err
	}

	if err := client.Create(ctx, clusterRole); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		existingCfg, err := getClusterRole(ctx, saName, client)
		if err != nil {
			return err
		}
		clusterRole.ResourceVersion = existingCfg.ResourceVersion
		err = client.Update(ctx, clusterRole)
		if err != nil {
			return err
		}
		log.Info("Updated webhook server cluster role")
	} else {
		log.Info("Created webhook server cluster role")
	}

	return nil
}

func getSpecRoleBinding(saName string) (*v1.ClusterRole, error) {
	clusterRole := &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   saName,
			Labels: server.WebhookServerAppLabels(),
		},
		Rules: []v1.PolicyRule{
			{
				APIGroups: []string{
					"admissionregistration.k8s.io",
				},
				Resources: []string{
					"mutatingwebhookconfigurations",
					"validatingwebhookconfigurations",
				},
				Verbs: []string{
					"*",
				},
			},
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"serviceaccounts",
				},
				Verbs: []string{
					"get",
				},
			},
		},
	}

	return clusterRole, nil
}

func getClusterRole(ctx context.Context, saName string, client crclient.Client) (*v1.ClusterRole, error) {
	clusterRole := &v1.ClusterRole{}
	namespacedName := types.NamespacedName{
		Name: saName,
	}
	err := client.Get(ctx, namespacedName, clusterRole)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return clusterRole, nil
}
