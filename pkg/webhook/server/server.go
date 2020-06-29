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
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/devfile/devworkspace-operator/internal/cluster"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

const (
	webhookServerHost    = "0.0.0.0"
	webhookServerPort    = 8443
	webhookServerCertDir = "/apiserver.local.config/certificates/"
)

var log = logf.Log.WithName("webhook.server")
var webhookServer *webhook.Server
var CABundle []byte

func ConfigureWebhookServer(mgr manager.Manager, ctx context.Context) error {
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
			"    To make your workspaces more secure, please configure them." +
			"    Skipping setting up Webhook Server")
		return nil
	}

	webhookTLSKeyName, webhookTLSCrtName, err := findTlsCrtAndKey()
	if err != nil {
		return err
	}

	CABundle, err = ioutil.ReadFile(webhookServerCertDir + webhookTLSCrtName)
	if os.IsNotExist(err) {
		return errors.New("CA certificate is not found. Unable to setup webhook server")
	}
	if err != nil {
		return err
	}

	log.Info("Setting up webhook server")

	webhookServer = mgr.GetWebhookServer()

	webhookServer.Port = webhookServerPort
	webhookServer.Host = webhookServerHost
	webhookServer.CertDir = webhookServerCertDir
	webhookServer.CertName = webhookTLSCrtName
	webhookServer.KeyName = webhookTLSKeyName

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

func findTlsCrtAndKey() (key, crt string, err error) {
	// Mounted from service serving-CA secret: tls.key and tls.crt
	if _, err := os.Stat(filepath.Join(webhookServerCertDir, "tls.key")); err == nil {
		key = "tls.key"
		if _, err := os.Stat(filepath.Join(webhookServerCertDir, "tls.crt")); err != nil {
			return "", "", fmt.Errorf("could not find certificates for webhook: could not find tls.crt")
		}
		crt = "tls.crt"
		return key, crt, nil
	}
	// Mounted by OLM: apiserver.key and apiserver.crt
	if _, err := os.Stat(filepath.Join(webhookServerCertDir, "apiserver.key")); err == nil {
		key = "apiserver.key"
		if _, err := os.Stat(filepath.Join(webhookServerCertDir, "apiserver.crt")); err != nil {
			return "", "", fmt.Errorf("could not find certificates for webhook: could not find apiserver.crt")
		}
		crt = "apiserver.crt"
		return key, crt, nil
	}
	return "", "", fmt.Errorf("could not find certificates for webhook")
}
