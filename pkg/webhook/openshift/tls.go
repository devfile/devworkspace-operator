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

package webhook_openshift

import (
	"context"

	"github.com/devfile/devworkspace-operator/pkg/webhook/service"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/devfile/devworkspace-operator/webhook/server"

	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var log = logf.Log.WithName("webhook-openshift")

func SetupSecureService(client crclient.Client, ctx context.Context, namespace string) error {
	log.Info("Attempting to create the secure service")
	err := service.CreateOrUpdateSecureService(client, ctx, namespace, map[string]string{
		"service.beta.openshift.io/serving-cert-secret-name": server.WebhookServerTLSSecretName,
	})
	if err != nil {
		log.Info("Failed creating the secure service")
		return err
	}
	return nil
}
