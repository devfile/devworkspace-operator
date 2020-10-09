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

	"github.com/devfile/devworkspace-operator/pkg/kubernetes/tls"
	"github.com/devfile/devworkspace-operator/webhook/server"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var log = logf.Log.WithName("webhook-k8s")

const (
	WebhookServerCACertSecretName = "devworkspace-webhookserver-ca-cert"
)

// SetupSecureService handles TLS secrets required for deployment on Kubernetes.
func SetupSecureService(client crclient.Client, ctx context.Context, namespace string) error {
	devworkspaceSecret := &corev1.Secret{}
	//check only tls certs because webhook server does not care about CA cert since it webhooks can be configured with non-CA domain cert
	err := client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: server.WebhookServerTLSSecretName}, devworkspaceSecret)
	if err != nil {
		if !errors.IsNotFound(err) {
			log.Error(err, "Error getting TLS secret "+server.WebhookServerTLSSecretName)
			return err
		} else {
			// TLS secret doesn't exist so we need to generate a new one
			err = tls.GenerateCerts(client, ctx, tls.GenCertParams{
				RequesterName: "webhook-server",
				Namespace:     namespace,
				CASecretName:  WebhookServerCACertSecretName,
				TLSSecretName: server.WebhookServerTLSSecretName,
				Domain:        server.WebhookServerServiceName + "." + namespace + ".svc",
			})
			if err != nil {
				return err
			}
		}
	}

	err = service.CreateOrUpdateSecureService(client, ctx, namespace, map[string]string{})
	if err != nil {
		log.Info("Failed creating the secure service")
		return err
	}

	return nil
}
