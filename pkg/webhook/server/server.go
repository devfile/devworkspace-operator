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
	"github.com/che-incubator/che-workspace-operator/internal/cluster"
	"github.com/che-incubator/che-workspace-operator/pkg/config"
	"io/ioutil"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

const (
	webhookServerHost    = "0.0.0.0"
	webhookServerPort    = 8443
	webhookServerCertDir = "/tmp/k8s-webhook-server/serving-certs"
)

var log = logf.Log.WithName("webhook.server")
var webhookServer *webhook.Server
var CABundle []byte

func ConfigureWebhookServer(mgr manager.Manager) error {
	if config.ControllerCfg.GetWebhooksEnabled() == "false" {
		log.Info("Webhooks are disabled. Skipping setting up webhook server")
		return nil
	}

	enabled, err := cluster.IsWebhookConfigurationEnabled()

	if err != nil {
		log.Info("ERROR: Could not evaluate if admission webhook configurations are available", "error", err)
		return err
	}

	if !enabled {
		log.Info("WARN: AdmissionWebhooks are not configured at your cluster." +
			"    To make your workspaces more secure, please configuring them." +
			"    Skipping setting up Webhook Server")
		return nil
	}

	CABundle, err = ioutil.ReadFile(webhookServerCertDir + "/ca.crt")
	if os.IsNotExist(err) {
		log.Info("CA certificate is not found. Webhook server is not set up")
		return nil
	}
	if err != nil {
		return err
	}

	log.Info("Setting up webhook server")

	webhookServer = mgr.GetWebhookServer()

	webhookServer.Port = webhookServerPort
	webhookServer.Host = webhookServerHost
	webhookServer.CertDir = webhookServerCertDir

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
