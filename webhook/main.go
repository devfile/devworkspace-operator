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
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/operator-framework/operator-sdk/pkg/log/zap"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/devfile/devworkspace-operator/webhook/workspace"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/devfile/devworkspace-operator/webhook/server"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var log = logf.Log.WithName("cmd")

func main() {
	logf.SetLogger(zap.Logger())

	clusterConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Error(err, "Failed when attempting to retrieve in cluster config")
		os.Exit(1)
	}

	namespace, err := k8sutil.GetOperatorNamespace()
	if err != nil {
		log.Error(err, "Failed to get Operator Namespace")
		os.Exit(1)
	}

	err = createWebhooks(clusterConfig, namespace)
	if err != nil {
		log.Error(err, "Failed to get create webhooks")
		os.Exit(1)
	}

	var shutdownChan = make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGTERM)

	log.Info("Starting webhook server")
	if err := server.GetWebhookServer().Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "Webhook server exited non-zero")
		os.Exit(1)
	}
}

func createWebhooks(clusterConfig *rest.Config, namespace string) error {
	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(clusterConfig, manager.Options{
		Namespace: namespace,
	})
	if err != nil {
		return err
	}

	log.Info("Configuring Webhook Server")
	err = server.ConfigureWebhookServer(mgr)
	if err != nil {
		return err
	}

	log.Info("Configuring Webhooks")
	if err := workspace.Configure(context.TODO()); err != nil {
		return err
	}
	return nil
}
