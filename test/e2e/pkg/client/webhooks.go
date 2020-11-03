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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (w *K8sClient) WaitForMutatingWebhooksConfigurations(name string) (deployed bool, err error) {
	timeout := time.After(15 * time.Minute)
	tick := time.Tick(2 * time.Second)

	for {
		select {
		case <-timeout:
			return false, errors.New("timed out")
		case <-tick:
			_, err := w.kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(context.TODO(), name, metav1.GetOptions{})
			if err == nil {
				return true, nil
			}
		}
	}
}
