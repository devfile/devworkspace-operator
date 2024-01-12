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

package controllers_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	dwv1 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha1"
	dwv2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/workspace/internal/testutil"
	kubesync "github.com/devfile/devworkspace-operator/pkg/library/kubernetes"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"

	workspacecontroller "github.com/devfile/devworkspace-operator/controllers/workspace"
	"github.com/devfile/devworkspace-operator/pkg/cache"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	testNamespace    = "devworkspace-test"
	devWorkspaceName = "test-devworkspace"
)

var (
	k8sClient         client.Client
	testEnv           *envtest.Environment
	ctx               context.Context
	cancel            context.CancelFunc
	testControllerCfg = config.GetConfigForTesting(&controllerv1alpha1.OperatorConfiguration{
		Workspace: &controllerv1alpha1.WorkspaceConfig{},
		Routing: &controllerv1alpha1.RoutingConfig{
			ClusterHostSuffix: "test-environment-cluster-suffix",
		},
		EnableExperimentalFeatures: pointer.Bool(true),
	})
)

func TestAPIs(t *testing.T) {
	if os.Getenv("SKIP_CONTROLLER_TESTS") == "true" {
		t.Skip()
	}

	RegisterFailHandler(Fail)

	RunSpecs(t, "DevWorkspace Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("setting up controller environment")
	Expect(setupEnvVars()).To(Succeed())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "deploy", "templates", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: filepath.Join("..", "..", "bin", "k8s", "1.24.2-linux-amd64"),
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	config.SetGlobalConfigForTesting(testControllerCfg)

	err = controllerv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = dwv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = dwv2.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// Replicate controller setup similarly to how we do for main.go
	err = kubesync.InitializeDeserializer(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	cacheFunc, err := cache.GetCacheFunc()
	Expect(err).NotTo(HaveOccurred())

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:   scheme.Scheme,
		Port:     9443,
		NewCache: cacheFunc,
	})
	Expect(err).NotTo(HaveOccurred())

	nonCachingClient, err := client.New(mgr.GetConfig(), client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	err = mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Event{}, "involvedObject.name", func(obj client.Object) []string {
		ev := obj.(*corev1.Event)
		return []string{ev.InvolvedObject.Name}
	})
	Expect(err).NotTo(HaveOccurred())

	// Don't set up DevWorkspaceRouting Reconciler so that we can manage routings

	err = (&workspacecontroller.DevWorkspaceReconciler{
		Client:           mgr.GetClient(),
		NonCachingClient: nonCachingClient,
		Log:              ctrl.Log.WithName("controllers").WithName("DevWorkspace"),
		Scheme:           mgr.GetScheme(),
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	// Set HTTP client to fail all requests by default; tests that require HTTP must set this up directly
	workspacecontroller.SetupHttpClientsForTesting(getBasicTestHttpClient())

	// Skip trying to set up / test webhooks for now

	By("Creating Namespace for the DevWorkspace")
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace,
		},
	}
	Expect(k8sClient.Create(ctx, ns)).Should(Succeed())

	go func() {
		defer GinkgoRecover()
		err = mgr.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func setupEnvVars() error {
	bytes, err := os.ReadFile(filepath.Join("..", "..", "deploy", "templates", "components", "manager", "manager.yaml"))
	if err != nil {
		return err
	}
	deploy := &appsv1.Deployment{}
	if err := yaml.Unmarshal(bytes, deploy); err != nil {
		return err
	}

	var dwContainer *corev1.Container
	for _, container := range deploy.Spec.Template.Spec.Containers {
		if container.Name == "devworkspace-controller" {
			dwContainer = &container
			break
		}
	}
	if dwContainer == nil {
		return fmt.Errorf("could not read devworkspace-controller container from manager.yaml")
	}

	for _, envvar := range dwContainer.Env {
		if err := os.Setenv(envvar.Name, envvar.Value); err != nil {
			return err
		}
	}

	return nil
}

func getBasicTestHttpClient() *http.Client {
	return &http.Client{
		Transport: &testutil.TestRoundTripper{},
	}
}
