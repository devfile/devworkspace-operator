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

	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"syscall"

	workspacev1alpha1 "github.com/devfile/api/pkg/apis/workspaces/v1alpha1"
	workspacev1alpha2 "github.com/devfile/api/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/internal/cluster"
	"github.com/devfile/devworkspace-operator/webhook/server"
	"github.com/devfile/devworkspace-operator/webhook/workspace"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

var (
	scheme = runtime.NewScheme()
	log    = logf.Log.WithName("cmd")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(workspacev1alpha1.AddToScheme(scheme))
	utilruntime.Must(workspacev1alpha2.AddToScheme(scheme))
}

func main() {
	logf.SetLogger(zap.New(zap.UseDevMode(true)))

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	namespace, err := cluster.GetWatchNamespace()
	if err != nil {
		log.Error(err, "Failed to get watch namespace")
		os.Exit(1)
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Namespace: namespace,
		Scheme:    scheme,
		CertDir:   server.WebhookServerCertDir,
	})
	if err != nil {
		log.Error(err, "Failed to create manager")
		os.Exit(1)
	}

	err = createWebhooks(mgr)
	if err != nil {
		log.Error(err, "Failed to create webhooks")
		os.Exit(1)
	}

	if err := ctrl.NewWebhookManagedBy(mgr).For(&workspacev1alpha1.DevWorkspace{}).Complete(); err != nil {
		log.Error(err, "failed creating conversion webhook")
	}
	if err := ctrl.NewWebhookManagedBy(mgr).For(&workspacev1alpha2.DevWorkspace{}).Complete(); err != nil {
		log.Error(err, "failed creating conversion webhook")
	}

	var shutdownChan = make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGTERM)

	log.Info("Starting manager")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "Manager exited non-zero")
		os.Exit(1)
	}
}

func createWebhooks(mgr manager.Manager) error {
	log.Info("Configuring Webhook Server")
	err := server.ConfigureWebhookServer(mgr)
	if err != nil {
		return err
	}

	log.Info("Configuring Webhooks")
	if err := workspace.Configure(context.TODO()); err != nil {
		return err
	}
	return nil
}
