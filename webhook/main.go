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
	"github.com/devfile/devworkspace-operator/webhook/server"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"k8s.io/client-go/rest"
	"log"
	"os"
	"os/signal"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"syscall"
)

// configureWebhookTasks is a list of functions to add set webhook up and add them to the Manager
var configureWebhookTasks []func(context.Context) error

func main() {
	log.SetOutput(os.Stdout)

	clusterConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal("Failed when attempting to retrieve in cluster config: ", err)
	}

	namespace, err := k8sutil.GetOperatorNamespace()
	if err != nil {
		log.Fatal(err)
	}

	err = createWebhooks(clusterConfig, namespace)
	if err != nil {
		log.Fatal(err)
	}

	var shutdownChan = make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGTERM)
	<-shutdownChan
}


func createWebhooks(clusterConfig *rest.Config, namespace string) error {
	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(clusterConfig, manager.Options{
		Namespace: namespace,
	})
	if err != nil {
		return err
	}

	log.Print("Configuring Webhook Server")
	err = server.ConfigureWebhookServer(mgr)
	if err != nil {
		return err
	}

	log.Print("Configuring Webhooks")
	for _, f := range configureWebhookTasks {
		if err := f(context.TODO()); err != nil {
			return err
		}
	}
	return nil
}