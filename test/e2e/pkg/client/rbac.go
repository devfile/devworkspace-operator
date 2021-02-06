//
// Copyright (c) 2019-2021 Red Hat, Inc.
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
	"log"
	"time"

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

//WaitSAToken waits until a secret with the token related to the specified SA
//error is returned if token is not found after 10 seconds of tries
func (w *K8sClient) WaitSAToken(namespace, serviceAccount string) (token string, err error) {
	var delay time.Duration = 1
	//usually the Service Account token is available just after SA is created but sometimes it's not
	//trying 10 seconds for stability reason
	var timeout time.Duration = 10
	left := timeout

	timeoutC := time.After(timeout * time.Second)
	tickC := time.Tick(delay * time.Second)

	for {
		select {
		case <-timeoutC:
			return "", errors.New(fmt.Sprintf("ServiceAccount '%s/%s' token is not found after %d", namespace, serviceAccount, timeout))
		case <-tickC:
			token, err = w.getSAToken(namespace, serviceAccount)

			if err != nil {
				return "", err
			}
			if token != "" {
				return token, nil
			}

			left--
			log.Printf("ServiceAccount '%s/%s' token is not found yet. Waiting %ds until it's removed. Will time out in %ds", namespace, serviceAccount, delay, left)
		}
	}
}

func (w *K8sClient) getSAToken(namespace string, serviceAccount string) (string, error) {
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

	return "", nil
}
