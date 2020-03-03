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
	"github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/config"
	"io/ioutil"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

const (
	webhookServerHost    = "0.0.0.0"
	webhookServerPort    = 8443
	webhookServerCertDir = "/tmp/k8s-webhook-server/serving-certs"
)

var log = logf.Log.WithName("webhook.server")

var CABundle []byte

func ConfigureWebhookServer(mgr manager.Manager) (bool, error) {
	if config.ControllerCfg.GetWebhooksEnabled() == "false" {
		return false, nil
	}

	enabled, err := cluster.IsWebhookConfigurationEnabled()

	if err != nil {
		log.Info("ERROR: Could not evaluate if admission webhook configurations are available", "error", err)
		return false, err
	}

	if !enabled {
		log.Info("WARN: AdmissionWebhooks are not configured at your cluster." +
			"    To make your workspaces more secure, please configuring them." +
			"    Skipping setting up Webhook Server")
		return false, nil
	}

	CABundle, err = ioutil.ReadFile(webhookServerCertDir + "/ca.crt")
	if os.IsNotExist(err) {
		log.Info("CA certificate is not found. Webhook server is not set up")
		return false, nil
	}
	if err != nil {
		return false, err
	}

	log.Info("Setting up webhook server")
	mgr.GetWebhookServer().Port = webhookServerPort
	mgr.GetWebhookServer().Host = webhookServerHost
	mgr.GetWebhookServer().CertDir = webhookServerCertDir

	return true, nil
}
