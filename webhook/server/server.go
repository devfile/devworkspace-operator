//
// Copyright (c) 2019-2024 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package server

import (
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

const (
	webhookServerHost    = "0.0.0.0"
	WebhookServerPort    = 8443
	WebhookServerCertDir = "/tmp/k8s-webhook-server/serving-certs"

	WebhookServerAppName        = "devworkspace-webhook-server"
	WebhookServerDeploymentName = WebhookServerAppName

	WebhookServerServiceName = "devworkspace-webhookserver"
	WebhookServerPortName    = "webhook-server"

	WebhookMetricsPortName = "metrics"

	// Holds webhook server related SA name and SA-related objects, like ClusterRole, ClusterRoleBinding
	WebhookServerSAName = "devworkspace-webhook-server"

	WebhookServerCertsVolumeName = "webhook-tls-certs"
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

var WebhookServerAppAnnotations = func() map[string]string {
	//Add restart timestamp which will update the webhook server
	//deployment to force restart. This is done so that the
	//serviceaccount uid is updated to use the latest and the
	//web-terminal does not hang.
	now := time.Now()
	return map[string]string{
		constants.WebhookRestartedAtAnnotation: strconv.FormatInt(now.UnixNano(), 10),
	}
}

func ConfigureWebhookServer(mgr manager.Manager) error {
	enabled, err := infrastructure.IsWebhookConfigurationEnabled()

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

	CABundle, err = os.ReadFile(WebhookServerCertDir + "/tls.crt")
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

// GetWebhookServer returns webhook server if it's configured, or nil otherwise
func GetWebhookServer() *webhook.Server {
	return webhookServer
}

// IsSetUp returns true if webhook server is configured, or false otherwise
func IsSetUp() bool {
	return webhookServer != nil
}
