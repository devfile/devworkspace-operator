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

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//CreateNamespace creates a new namespace
func (w *K8sClient) CreateNamespace(namespace string) error {
	_, err := w.kubeClient.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}, metav1.CreateOptions{})
	if k8sErrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

//DeleteNamespace deletes a namespace
func (w *K8sClient) DeleteNamespace(namespace string) error {
	return w.kubeClient.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
}

//WaitNamespaceIsTerminated waits until namespace that is marked to be removed, is fully cleaned up
func (w *K8sClient) WaitNamespaceIsTerminated(namespace string) (err error) {
	var delay time.Duration = 1
	var timeout time.Duration = 60
	left := timeout

	timeoutC := time.After(timeout * time.Second)
	tickC := time.Tick(delay * time.Second)

	for {
		select {
		case <-timeoutC:
			return errors.New(fmt.Sprintf("The namespace %s is not terminated and removed after %ds.", namespace, timeout))
		case <-tickC:
			var ns *corev1.Namespace
			ns, err = w.kubeClient.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})
			if err != nil {
				if k8sErrors.IsNotFound(err) {
					return nil
				}
				log.Printf("Failed to get namespace '%s'to verify if it's fully removed, will try again. Err: %s", namespace, err.Error())
			}
			left--
			log.Printf("Namespace '%s' is in %s phase. Waiting %ds until it's removed. Will time out in %ds", namespace, ns.Status.Phase, delay, left)
		}
	}
}
