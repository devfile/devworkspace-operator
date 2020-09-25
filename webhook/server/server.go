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

	WebhookServerDeploymentName = WebhookServerAppName

	WebhookServerAppName = "devworkspace-webhook-server"

	WebhookServerServiceName = "devworkspace-webhookserver"
	WebhookServerPortName    = "webhook-server"

	// Holds webhook server related SA name and SA-related objects, like ClusterRole, ClusterRoleBinding
	WebhookServerSAName = "devworkspace-webhook-server"

	WebhookServerCertsVolumeName = "webhook-tls-certs"

	//Secret name with TLS certs inside (tls.crt + tls.key) that is mounted to webhook server
	WebhookServerTLSSecretName = "devworkspace-webhookserver-tls"
)

var log = logf.Log.WithName("webhook.server")
var webhookServer *webhook.Server
var CABundle []byte

var WebhookServerAppLabels = func() map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":    WebhookServerAppName,
		"app.kubernetes.io/part-of": "devworkspace-operator",
	}
}

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

	CABundle, err = ioutil.ReadFile(WebhookServerCertDir + "/tls.crt")
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
