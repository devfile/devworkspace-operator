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

package client

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (w *K8sClient) CreateSA(name, namespace string) error {
	err := w.crClient.Create(context.TODO(), &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	})
	if k8sErrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func (w *K8sClient) AssignRoleToSA(namespace, serviceAccount, role string) error {
	err := w.crClient.Create(context.TODO(), &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      role + "2" + serviceAccount,
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Name:      serviceAccount,
				Namespace: namespace,
				Kind:      "ServiceAccount",
			},
		},
		RoleRef: rbacv1.RoleRef{
			Name: role,
			Kind: "ClusterRole",
		},
	})
	if k8sErrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func (w *K8sClient) GetSAToken(namespace, serviceAccount string) (string, error) {
	secrets, err := w.kubeClient.CoreV1().Secrets(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	for _, secret := range secrets.Items {
		if secret.Type == corev1.SecretTypeServiceAccountToken &&
			secret.Annotations[corev1.ServiceAccountNameKey] == serviceAccount {
			token, present := secret.Data["token"]
			if !present {
				continue
			}
			return string(token), nil
		}
	}
	return "", errors.New(fmt.Sprintf("token for service account '%s' is not found", serviceAccount))
}
