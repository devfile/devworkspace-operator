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

// CreateNamespace creates a new namespace
func (w *K8sClient) CreateNamespace(namespace string) error {
	_, err := w.kubeClient.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}, metav1.CreateOptions{})
	if k8sErrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

// DeleteNamespace deletes a namespace
func (w *K8sClient) DeleteNamespace(namespace string) error {
	return w.kubeClient.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
}

// WaitNamespaceIsTerminated waits until namespace that is marked to be removed, is fully cleaned up
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
