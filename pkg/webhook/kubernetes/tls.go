//
// Copyright (c) 2012-2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package webhook_k8s

import (
	"context"

	"github.com/devfile/devworkspace-operator/pkg/webhook/service"

	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("webhook-k8s")

// SetupSecureService handles TLS secrets required for deployment on Kubernetes.
func SetupSecureService(client crclient.Client, ctx context.Context, secretName, namespace string) error {
	err := service.CreateOrUpdateSecureService(client, ctx, namespace, map[string]string{})
	if err != nil {
		log.Info("Failed creating the secure service")
		return err
	}

	return nil
}
