//
// Copyright (c) 2019-2021 Red Hat, Inc.
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
package config

import (
	"fmt"
	"os"
	"testing"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/google/go-cmp/cmp"
	fuzz "github.com/google/gofuzz"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
)

const testNamespace = "test-namespace"

var (
	scheme   = runtime.NewScheme()
	trueBool = true
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(dw.AddToScheme(scheme))
	utilruntime.Must(routev1.Install(scheme))
}

func setupForTest(t *testing.T) {
	if err := os.Setenv("WATCH_NAMESPACE", testNamespace); err != nil {
		t.Fatalf("failed to set up for test: %s", err)
	}
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	t.Cleanup(func() { internalConfig = nil })
}

func TestSetupControllerConfigUsesDefault(t *testing.T) {
	setupForTest(t)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	err := SetupControllerConfig(client)
	if !assert.NoError(t, err, "Should not return error") {
		return
	}
	assert.Equal(t, DefaultConfig, internalConfig, "Config used should be the default")
}

func TestSetupControllerDefaultsAreExported(t *testing.T) {
	setupForTest(t)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	err := SetupControllerConfig(client)
	if !assert.NoError(t, err, "Should not return error") {
		return
	}
	assert.Equal(t, DefaultConfig.Routing, Routing, "Configuration should be exported")
	assert.Equal(t, DefaultConfig.Workspace, Workspace, "Configuration should be exported")
}

func TestSetupControllerConfigFailsWhenAlreadySetup(t *testing.T) {
	setupForTest(t)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	err := SetupControllerConfig(client)
	if !assert.NoError(t, err, "Should not return error") {
		return
	}
	err = SetupControllerConfig(client)
	if !assert.Error(t, err, "Should return error when config is already setup") {
		return
	}
	assert.Equal(t, DefaultConfig, internalConfig, "Config used should be the default")
}

func TestSetupControllerMergesClusterConfig(t *testing.T) {
	setupForTest(t)

	clusterConfig := buildConfig(&v1alpha1.OperatorConfiguration{
		Routing: &v1alpha1.RoutingConfig{
			DefaultRoutingClass: "test-routingClass",
			ClusterHostSuffix:   "192.168.0.1.nip.io",
		},
		Workspace: &v1alpha1.WorkspaceConfig{
			ImagePullPolicy: "IfNotPresent",
		},
		EnableExperimentalFeatures: &trueBool,
	})
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clusterConfig).Build()

	expectedConfig := DefaultConfig.DeepCopy()
	expectedConfig.Routing.DefaultRoutingClass = "test-routingClass"
	expectedConfig.Routing.ClusterHostSuffix = "192.168.0.1.nip.io"
	expectedConfig.Workspace.ImagePullPolicy = "IfNotPresent"
	expectedConfig.EnableExperimentalFeatures = &trueBool

	err := SetupControllerConfig(client)
	if !assert.NoError(t, err, "Should not return error") {
		return
	}
	assert.Equal(t, expectedConfig, internalConfig, fmt.Sprintf("Processed config should merge settings from cluster: %s", cmp.Diff(internalConfig, expectedConfig)))
	assert.Equal(t, internalConfig.Routing, Routing, fmt.Sprintf("Changes to config should be propagated to exported fields"))
	assert.Equal(t, internalConfig.Workspace, Workspace, fmt.Sprintf("Changes to config should be propagated to exported fields"))
}

func TestDetectsOpenShiftRouteSuffix(t *testing.T) {
	setupForTest(t)
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	testRoute := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      openShiftTestRouteName,
			Namespace: testNamespace,
		},
		Spec: routev1.RouteSpec{
			Host: "test-host",
		},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(testRoute).Build()
	err := SetupControllerConfig(client)
	if !assert.NoError(t, err, "Should not return error") {
		return
	}
	assert.Equal(t, "test-host", internalConfig.Routing.ClusterHostSuffix, "Should determine host suffix with route on OpenShift")
}

func TestSyncConfigFromIgnoresOtherConfigInOtherNamespaces(t *testing.T) {
	setupForTest(t)
	internalConfig = DefaultConfig.DeepCopy()
	config := buildConfig(&v1alpha1.OperatorConfiguration{
		Workspace: &v1alpha1.WorkspaceConfig{
			ImagePullPolicy: "IfNotPresent",
		},
	})
	config.Namespace = "not-test-namespace"
	syncConfigFrom(config)
	assert.Equal(t, DefaultConfig, internalConfig)
}

func TestSyncConfigFromIgnoresOtherConfigWithOtherName(t *testing.T) {
	setupForTest(t)
	internalConfig = DefaultConfig.DeepCopy()
	config := buildConfig(&v1alpha1.OperatorConfiguration{
		Workspace: &v1alpha1.WorkspaceConfig{
			ImagePullPolicy: "IfNotPresent",
		},
	})
	config.Name = "testing-name"
	syncConfigFrom(config)
	assert.Equal(t, DefaultConfig, internalConfig)
}

func TestSyncConfigDoesNotChangeDefaults(t *testing.T) {
	setupForTest(t)
	oldDefaultConfig := DefaultConfig.DeepCopy()
	internalConfig = DefaultConfig.DeepCopy()
	config := buildConfig(&v1alpha1.OperatorConfiguration{
		Routing: &v1alpha1.RoutingConfig{
			DefaultRoutingClass: "test-routingClass",
			ClusterHostSuffix:   "192.168.0.1.nip.io",
		},
		Workspace: &v1alpha1.WorkspaceConfig{
			ImagePullPolicy: "IfNotPresent",
		},
		EnableExperimentalFeatures: &trueBool,
	})
	syncConfigFrom(config)
	internalConfig.Routing.DefaultRoutingClass = "Changed after the fact"
	assert.Equal(t, DefaultConfig, oldDefaultConfig)
}

func TestSyncConfigDoesNotEraseClusterRoutingSuffix(t *testing.T) {
	setupForTest(t)
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	testRoute := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      openShiftTestRouteName,
			Namespace: testNamespace,
		},
		Spec: routev1.RouteSpec{
			Host: fmt.Sprintf("%s-%s.test-host", openShiftTestRouteName, testNamespace),
		},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(testRoute).Build()
	err := SetupControllerConfig(client)
	if !assert.NoError(t, err, "Should not return error") {
		return
	}
	assert.Equal(t, "test-host", Routing.ClusterHostSuffix, "Should get clusterHostSuffix from route on OpenShift")
	syncConfigFrom(buildConfig(&v1alpha1.OperatorConfiguration{
		Routing: &v1alpha1.RoutingConfig{
			DefaultRoutingClass: "test-routingClass",
		},
		Workspace: &v1alpha1.WorkspaceConfig{
			ImagePullPolicy: "IfNotPresent",
		},
	}))
	if !assert.NoError(t, err, "Should not return error") {
		return
	}
	assert.Equal(t, "test-host", Routing.ClusterHostSuffix, "clusterHostSuffix should persist after an update")
}

func TestMergeConfigLooksAtAllFields(t *testing.T) {
	f := fuzz.New().NilChance(0)
	expectedConfig := &v1alpha1.OperatorConfiguration{}
	actualConfig := &v1alpha1.OperatorConfiguration{}
	f.Fuzz(expectedConfig)
	mergeConfig(expectedConfig, actualConfig)
	assert.Equal(t, expectedConfig, actualConfig, "merging configs should merge all fields")
}

func buildConfig(config *v1alpha1.OperatorConfiguration) *v1alpha1.DevWorkspaceOperatorConfig {
	return &v1alpha1.DevWorkspaceOperatorConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OperatorConfigName,
			Namespace: testNamespace,
		},
		Config: config,
	}
}
