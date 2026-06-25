//
// Copyright (c) 2019-2025 Red Hat, Inc.
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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"

	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"

	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	dwv1 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha1"
	dwv2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/cache"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/devfile/devworkspace-operator/pkg/tlssetup"
	"github.com/devfile/devworkspace-operator/version"
	"github.com/devfile/devworkspace-operator/webhook/server"
	"github.com/devfile/devworkspace-operator/webhook/workspace"

	configv1 "github.com/openshift/api/config/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	clientconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

var (
	scheme = k8sruntime.NewScheme()
	log    = logf.Log.WithName("cmd")
)

func init() {
	// Figure out if we're running on OpenShift
	err := infrastructure.Initialize()
	if err != nil {
		log.Error(err, "could not determine cluster type")
		os.Exit(1)
	}

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(dwv1.AddToScheme(scheme))
	utilruntime.Must(dwv2.AddToScheme(scheme))

	if infrastructure.IsOpenShift() {
		// Needed for SecurityProfileWatcher to reconcile APIServer objects
		utilruntime.Must(configv1.AddToScheme(scheme))
	}
}

func main() {
	logf.SetLogger(zap.New(zap.UseDevMode(config.GetDevModeEnabled())))

	var metricsAddr string
	var tlsMinVersion string
	var tlsCipherSuites string
	flag.StringVar(&metricsAddr, "metrics-addr", ":9443", "The address the metric endpoint binds to.")
	flag.StringVar(&tlsMinVersion, "tls-min-version", "",
		"Minimum TLS version for metrics and webhook servers (e.g. VersionTLS12). "+
			"Overrides the OpenShift cluster TLS security profile when set.")
	flag.StringVar(&tlsCipherSuites, "tls-cipher-suites", "",
		"Comma-separated list of TLS cipher suites for metrics and webhook servers. "+
			"Overrides the OpenShift cluster TLS security profile when set.")
	flag.Parse()

	// Print versions
	log.Info(fmt.Sprintf("Operator Version: %s", version.Version))
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	log.Info(fmt.Sprintf("Commit: %s", version.Commit))
	log.Info(fmt.Sprintf("BuildTime: %s", version.BuildTime))

	// Get a config to talk to the apiserver
	cfg, err := clientconfig.GetConfig()
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	serverTLS, err := tlssetup.BuildServerTLSOptions(
		context.Background(), cfg, scheme, tlsMinVersion, tlsCipherSuites, log)
	if err != nil {
		log.Error(err, "failed to build TLS options for servers")
		os.Exit(1)
	}
	tlsOptsForServers := serverTLS.TLSOpts

	namespace, err := infrastructure.GetWatchNamespace()
	if err != nil {
		log.Error(err, "Failed to get watch namespace")
		os.Exit(1)
	}

	cacheFunc, err := cache.GetWebhooksCacheFunc(namespace)
	if err != nil {
		log.Error(err, "failed to set up objects cache")
		os.Exit(1)
	}

	webhookServer := webhook.NewServer(webhook.Options{
		CertDir: server.WebhookServerCertDir,
		Port:    server.WebhookServerPort,
		Host:    server.WebhookServerHost,
		TLSOpts: tlsOptsForServers,
	})

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:    metricsAddr,
			FilterProvider: filters.WithAuthenticationAndAuthorization,
			SecureServing:  true,
			TLSOpts:        tlsOptsForServers,
		},
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: ":6789",
		NewCache:               cacheFunc,
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

	// On OpenShift, watch cluster TLS profile and restart if it changes (unless overridden by CLI flags).
	signalCtx := signals.SetupSignalHandler()
	ctx, cancelCtx := context.WithCancel(signalCtx)
	defer cancelCtx()

	if err := tlssetup.RegisterSecurityProfileWatcher(mgr, serverTLS, cancelCtx, log); err != nil {
		log.Error(err, "unable to set up TLS security profile watcher")
		os.Exit(1)
	}

	// Setup health check
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "Unable to set up health check")
		os.Exit(1)
	}

	// Setup ready check
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error(err, "Unable to set up ready check")
		os.Exit(1)
	}

	log.Info("Starting manager")
	if err := mgr.Start(ctx); err != nil {
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
	if err := workspace.Configure(context.TODO(), mgr); err != nil {
		return err
	}
	return nil
}
