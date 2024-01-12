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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/devfile/devworkspace-operator/controllers/controller/devworkspacerouting"
	"github.com/devfile/devworkspace-operator/controllers/controller/devworkspacerouting/solvers"
	"github.com/devfile/devworkspace-operator/pkg/cache"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	kubesync "github.com/devfile/devworkspace-operator/pkg/library/kubernetes"
	"github.com/devfile/devworkspace-operator/pkg/webhook"
	"github.com/devfile/devworkspace-operator/version"

	dwv1 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha1"
	dwv2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	workspacecontroller "github.com/devfile/devworkspace-operator/controllers/workspace"

	configv1 "github.com/openshift/api/config/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	securityv1 "github.com/openshift/api/security/v1"
	templatev1 "github.com/openshift/api/template/v1"
	corev1 "k8s.io/api/core/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = k8sruntime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	// Figure out if we're running on OpenShift
	err := infrastructure.Initialize()
	if err != nil {
		setupLog.Error(err, "could not determine cluster type")
		os.Exit(1)
	}

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(controllerv1alpha1.AddToScheme(scheme))
	utilruntime.Must(dwv1.AddToScheme(scheme))
	utilruntime.Must(dwv2.AddToScheme(scheme))

	if infrastructure.IsOpenShift() {
		utilruntime.Must(routev1.Install(scheme))
		utilruntime.Must(templatev1.Install(scheme))
		utilruntime.Must(oauthv1.Install(scheme))
		// Enable controller to manage SCCs in OpenShift; permissions to do this are not requested
		// by default and must be added by a cluster-admin.
		utilruntime.Must(securityv1.Install(scheme))
		// Enable controller to read cluster-wide proxy on OpenShift
		utilruntime.Must(configv1.AddToScheme(scheme))
	}

	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(config.GetDevModeEnabled())))

	// Print versions
	setupLog.Info(fmt.Sprintf("Operator Version: %s", version.Version))
	setupLog.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	setupLog.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	setupLog.Info(fmt.Sprintf("Commit: %s", version.Commit))
	setupLog.Info(fmt.Sprintf("BuildTime: %s", version.BuildTime))

	if err := kubesync.InitializeDeserializer(scheme); err != nil {
		setupLog.Error(err, "failed to initialized Kubernetes objects decoder")
	}

	cacheFunc, err := cache.GetCacheFunc()
	if err != nil {
		setupLog.Error(err, "failed to set up objects cache")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: ":6789",
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "8d217f93.devfile.io",
		NewCache:               cacheFunc,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
	if err = setupControllerConfig(mgr); err != nil {
		setupLog.Error(err, "unable to read controller configuration")
		os.Exit(1)
	}

	nonCachingClient, err := client.New(mgr.GetConfig(), client.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "unable to initialize non-caching client")
		os.Exit(1)
	}

	// Index Events on involvedObject.name to allow us to get events involving a DevWorkspace's pod(s). This is used to
	// check for issues that prevent the pod from starting, so that DevWorkspaces aren't just hanging indefinitely.
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Event{}, "involvedObject.name", func(obj client.Object) []string {
		ev := obj.(*corev1.Event)
		return []string{ev.InvolvedObject.Name}
	}); err != nil {
		setupLog.Error(err, "unable to update indexer to include event involvedObjects")
		os.Exit(1)
	}

	if err = (&devworkspacerouting.DevWorkspaceRoutingReconciler{
		Client:       mgr.GetClient(),
		Log:          ctrl.Log.WithName("controllers").WithName("DevWorkspaceRouting"),
		Scheme:       mgr.GetScheme(),
		SolverGetter: &solvers.SolverGetter{},
		DebugLogging: config.ExperimentalFeaturesEnabled(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DevWorkspaceRouting")
		os.Exit(1)
	}
	if err = (&workspacecontroller.DevWorkspaceReconciler{
		Client:           mgr.GetClient(),
		NonCachingClient: nonCachingClient,
		Log:              ctrl.Log.WithName("controllers").WithName("DevWorkspace"),
		Scheme:           mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DevWorkspace")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	// Get a config to talk to the apiserver
	cfg, err := ctrlconfig.GetConfig()
	if err != nil {
		setupLog.Error(err, "")
		os.Exit(1)
	}

	setupLog.Info("setting up webhooks")
	if err := webhook.SetupWebhooks(context.Background(), cfg); err != nil {
		setupLog.Error(err, "failed to setup webhooks")
		os.Exit(1)
	}

	if err := ctrl.NewWebhookManagedBy(mgr).For(&dwv1.DevWorkspace{}).Complete(); err != nil {
		setupLog.Error(err, "failed creating conversion webhook for DevWorkspaces v1alpha1")
	}
	if err := ctrl.NewWebhookManagedBy(mgr).For(&dwv2.DevWorkspace{}).Complete(); err != nil {
		setupLog.Error(err, "failed creating conversion webhook for DevWorkspaces v1alpha2")
	}

	// Setup health check
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "Unable to set up health check")
		os.Exit(1)
	}

	// Setup ready check
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "Unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func setupControllerConfig(mgr ctrl.Manager) error {
	nonCachedClient, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: mgr.GetScheme(),
	})
	if err != nil {
		return err
	}
	if err := config.MigrateConfigFromConfigMap(nonCachedClient); err != nil {
		return err
	}
	return config.SetupControllerConfig(nonCachedClient)
}
