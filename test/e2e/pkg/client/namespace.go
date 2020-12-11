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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//create a project under login user using oc client
func (w *K8sClient) CreateProjectWithKubernetesContext(projectName string) error {
	_, err := w.kubeClient.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: projectName}}, metav1.CreateOptions{})
	if errors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

//delete a project under login user using oc client
func (w *K8sClient) DeleteProjectWithKubernetesContext(projectName string) error {
	err := w.kubeClient.CoreV1().Namespaces().Delete(context.TODO(), projectName, metav1.DeleteOptions{})
	return err
}
