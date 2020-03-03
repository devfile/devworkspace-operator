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
package webhook

import (
	"context"
	"github.com/che-incubator/che-workspace-operator/pkg/webhook/server"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var log = logf.Log.WithName("webhook")

// configureWebhookTasks is a list of functions to add set webhook up and add them to the Manager
var configureWebhookTasks []func(*webhook.Server, context.Context) error

// SetUpWebhooks sets up Webhook server and registers webhooks configurations
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=mutatingwebhookconfigurations;validatingwebhookconfigurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
func SetUpWebhooks(mgr manager.Manager, ctx context.Context) error {
	success, err := server.ConfigureWebhookServer(mgr)
	if !success {
		if err != nil {
			return err
		} else {
			log.Info("Webhook server is not set up. Skipping webhook cfg registration")
			return nil
		}
	}

	for _, f := range configureWebhookTasks {
		if err := f(mgr.GetWebhookServer(), ctx); err != nil {
			return err
		}
	}
	return nil
}
