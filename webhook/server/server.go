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
package server

import (
	"errors"
	"io/ioutil"
	"os"

	"github.com/devfile/devworkspace-operator/internal/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

const (
	webhookServerHost    = "0.0.0.0"
	WebhookServerPort    = 8443
	WebhookServerCertDir = "/tmp/k8s-webhook-server/serving-certs"

	WebhookServerServiceName = "devworkspace-webhookserver"
	WebhookServerPortName    = "webhook-server"

	CertConfigMapName = "devworkspace-webhookserver-secure-service"
	CertSecretName    = "devworkspace-webhookserver"

	WebhookCertsVolumeName = "webhook-tls-certs"
)

var log = logf.Log.WithName("webhook.server")
var webhookServer *webhook.Server
var CABundle []byte

func ConfigureWebhookServer(mgr manager.Manager) error {
	enabled, err := cluster.IsWebhookConfigurationEnabled()

	if err != nil {
		log.Info("ERROR: Could not evaluate if admission webhook configurations are available", "error", err)
		return err
	}

	if !enabled {
		log.Info("WARN: AdmissionWebhooks are not configured at your cluster." +
			"    To make your workspaces more secure, please configure them." +
			"    Skipping setting up Webhook Server")
		return nil
	}

	CABundle, err = ioutil.ReadFile(WebhookServerCertDir + "/ca.crt")
	if os.IsNotExist(err) {
		return errors.New("CA certificate is not found. Unable to setup webhook server")
	}
	if err != nil {
		return err
	}

	log.Info("Setting up webhook server")

	webhookServer = mgr.GetWebhookServer()

	webhookServer.Port = WebhookServerPort
	webhookServer.Host = webhookServerHost
	webhookServer.CertDir = WebhookServerCertDir

	return nil
}

//GetWebhookServer returns webhook server if it's configured
//  nil otherwise
func GetWebhookServer() *webhook.Server {
	return webhookServer
}

//IsSetUp returns true if webhook server is configured
//  false otherwise
func IsSetUp() bool {
	return webhookServer != nil
}
