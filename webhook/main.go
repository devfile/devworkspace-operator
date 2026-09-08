//
// Copyright (c) 2019-2026 Red Hat, Inc.
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
	"os/signal"
	"runtime"
	"syscall"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/httpfactory"
	kubesync "github.com/devfile/devworkspace-operator/pkg/library/kubernetes"
	configv1 "github.com/openshift/api/config/v1"
	routev1 "github.com/openshift/api/route/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	dwv1 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha1"
	dwv2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/cache"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/devfile/devworkspace-operator/version"
	"github.com/devfile/devworkspace-operator/webhook/server"
	"github.com/devfile/devworkspace-operator/webhook/workspace"

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
	utilruntime.Must(controllerv1alpha1.AddToScheme(scheme))
	utilruntime.Must(dwv1.AddToScheme(scheme))
	utilruntime.Must(dwv2.AddToScheme(scheme))

	if infrastructure.IsOpenShift() {
		utilruntime.Must(routev1.Install(scheme))
		utilruntime.Must(configv1.Install(scheme))
	}
}

func main() {
	logf.SetLogger(zap.New(zap.UseDevMode(config.GetDevModeEnabled())))

	var metricsAddr string
	flag.StringVar(&metricsAddr, "metrics-addr", ":9443", "The address the metric endpoint binds to.")
	flag.Parse()

	// Print versions
	log.Info(fmt.Sprintf("Operator Version: %s", version.Version))
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	log.Info(fmt.Sprintf("Commit: %s", version.Commit))
	log.Info(fmt.Sprintf("BuildTime: %s", version.BuildTime))

	if err := kubesync.InitializeDeserializer(scheme); err != nil {
		log.Error(err, "Failed to initialize kubernetes object deserializer")
		os.Exit(1)
	}

	// Get a config to talk to the apiserver
	cfg, err := clientconfig.GetConfig()
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

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
	})

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:    metricsAddr,
			FilterProvider: filters.WithAuthenticationAndAuthorization,
			SecureServing:  true,
		},
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: ":6789",
		NewCache:               cacheFunc,
	})
	if err != nil {
		log.Error(err, "Failed to create manager")
		os.Exit(1)
	}

	nonCachedClient, err := client.New(mgr.GetConfig(), client.Options{Scheme: mgr.GetScheme()})
	if err != nil {
		log.Error(err, "Failed to setup nonCachedClient")
		os.Exit(1)
	}

	err = config.SetupControllerConfig(nonCachedClient)
	if err != nil {
		log.Error(err, "Failed to setup Controller Config")
		os.Exit(1)
	}

	err = httpfactory.SetupHttpClientsFactory(mgr.GetClient(), mgr.GetLogger())
	if err != nil {
		log.Error(err, "Failed to setup Http clients factory")
		os.Exit(1)
	}

	err = setupConfigCacheWatcher(mgr)
	if err != nil {
		log.Error(err, "Failed to setup config watcher")
		os.Exit(1)
	}

	err = createWebhooks(mgr)
	if err != nil {
		log.Error(err, "Failed to create webhooks")
		os.Exit(1)
	}

	var shutdownChan = make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGTERM)

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
	if err := workspace.Configure(context.TODO(), mgr); err != nil {
		return err
	}
	return nil
}

func setupConfigCacheWatcher(mgr ctrl.Manager) error {
	emptyMapper := func(ctx context.Context, obj client.Object) []reconcile.Request {
		return []reconcile.Request{}
	}

	// Do nothing, just for keeping DWOC up to date
	return ctrl.NewControllerManagedBy(mgr).
		Named("dwoc-config-watcher").
		WithOptions(controller.Options{
			UsePriorityQueue: ptr.To(false),
		}).
		Watches(&controllerv1alpha1.DevWorkspaceOperatorConfig{},
			handler.EnqueueRequestsFromMapFunc(emptyMapper),
			builder.WithPredicates(config.Predicates())).
		Complete(reconcile.Func(func(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
			return reconcile.Result{}, nil
		}))
}
