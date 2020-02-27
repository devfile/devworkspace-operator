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
	"github.com/che-incubator/che-workspace-operator/internal/cluster"
	"io/ioutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

const (
	webhookServerHost    = "0.0.0.0"
	webhookServerPort    = 8443
	webhookServerCertDir = "/tmp/k8s-webhook-server/serving-certs"
	webhookCADir         = "/tmp/k8s-webhook-server/certificate-authority"
)

var log = logf.Log.WithName("webhook.server")

var CABundle []byte

func ConfigureWebhookServer(mgr manager.Manager, ctx context.Context) (bool, error) {
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

	if inCluster, err := cluster.IsInCluster(); !inCluster || err != nil {
		if err != nil {
			return false, err
		}
		log.Info("Controller is run outside of cluster. Skipping setting webhook server up")
		return false, nil
	}

	if err := generateTLSCerts(mgr, ctx); err != nil {
		return false, err
	}

	CABundle, err = ioutil.ReadFile(webhookCADir + "/ca.crt")
	if err != nil {
		//after generating TLS certs first run will fail.
		//TODO Rework and read certs directly from configmap,secret to avoid rebooting
		return false, err
	}

	log.Info("Setting up webhook server")
	mgr.GetWebhookServer().Port = webhookServerPort
	mgr.GetWebhookServer().Host = webhookServerHost
	mgr.GetWebhookServer().CertDir = webhookServerCertDir

	return true, nil
}
