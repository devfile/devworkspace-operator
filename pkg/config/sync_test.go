//
// Copyright (c) 2019-2022 Red Hat, Inc.
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
	"context"
	"fmt"
	"testing"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	attributes "github.com/devfile/api/v2/pkg/attributes"
	"github.com/google/go-cmp/cmp"
	fuzz "github.com/google/gofuzz"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
)

func TestSetupControllerConfigUsesDefault(t *testing.T) {
	setupForTest(t)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	err := SetupControllerConfig(client)
	if !assert.NoError(t, err, "Should not return error") {
		return
	}
	assert.Equal(t, defaultConfig, internalConfig, "Config used should be the default")
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
	assert.Equal(t, defaultConfig, internalConfig, "Config used should be the default")
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

	expectedConfig := defaultConfig.DeepCopy()
	expectedConfig.Routing.DefaultRoutingClass = "test-routingClass"
	expectedConfig.Routing.ClusterHostSuffix = "192.168.0.1.nip.io"
	expectedConfig.Workspace.ImagePullPolicy = "IfNotPresent"
	expectedConfig.EnableExperimentalFeatures = &trueBool

	err := SetupControllerConfig(client)
	if !assert.NoError(t, err, "Should not return error") {
		return
	}
	assert.Equal(t, expectedConfig, internalConfig, fmt.Sprintf("Processed config should merge settings from cluster: %s", cmp.Diff(internalConfig, expectedConfig)))
}

func testCodeCoverageWorks(t *testing.T) {
	setupForTest(t)
	i := testedFunction()
	fmt.Printf("number is %d", i)
}

func TestCatchesNonExistentExternalDWOC(t *testing.T) {
	setupForTest(t)

	workspace := &dw.DevWorkspace{}
	attributes := attributes.Attributes{}
	namespacedName := types.NamespacedName{
		Name:      "external-config-name",
		Namespace: "external-config-namespace",
	}
	attributes.Put(constants.ExternalDevWorkspaceConfiguration, namespacedName, nil)
	workspace.Spec.Template.DevWorkspaceTemplateSpecContent = dw.DevWorkspaceTemplateSpecContent{
		Attributes: attributes,
	}
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	resolvedConfig, err := ResolveConfigForWorkspace(workspace, client)
	if !assert.Error(t, err, "Error should be given if external DWOC specified in workspace spec does not exist") {
		return
	}
	assert.Equal(t, resolvedConfig, internalConfig, "Internal/Global DWOC should be used as fallback if external DWOC could not be resolved")
}

func TestMergeExternalConfig(t *testing.T) {
	setupForTest(t)

	workspace := &dw.DevWorkspace{}
	attributes := attributes.Attributes{}
	namespacedName := types.NamespacedName{
		Name:      externalConfigName,
		Namespace: externalConfigNamespace,
	}
	attributes.Put(constants.ExternalDevWorkspaceConfiguration, namespacedName, nil)
	workspace.Spec.Template.DevWorkspaceTemplateSpecContent = dw.DevWorkspaceTemplateSpecContent{
		Attributes: attributes,
	}

	// External config is based off of the default/internal config, with just a few changes made
	// So when the internal config is merged with the external one, they will become identical
	externalConfig := buildExternalConfig(defaultConfig.DeepCopy())
	externalConfig.Config.Workspace.ImagePullPolicy = "Always"
	externalConfig.Config.Workspace.PVCName = "test-PVC-name"
	storageSize := resource.MustParse("15Gi")
	externalConfig.Config.Workspace.DefaultStorageSize = &v1alpha1.StorageSizes{
		Common:       &storageSize,
		PerWorkspace: &storageSize,
	}

	clusterConfig := buildConfig(defaultConfig.DeepCopy())
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clusterConfig, externalConfig).Build()
	err := SetupControllerConfig(client)
	if !assert.NoError(t, err, "Should not return error") {
		return
	}

	// Sanity check
	if !cmp.Equal(clusterConfig.Config, internalConfig) {
		t.Error("Internal config should be same as cluster config before starting:", cmp.Diff(clusterConfig.Config, internalConfig))
	}

	resolvedConfig, err := ResolveConfigForWorkspace(workspace, client)
	if !assert.NoError(t, err, "Should not return error") {
		return
	}

	// Compare the resolved config and external config
	if !cmp.Equal(resolvedConfig, externalConfig.Config) {
		t.Error("Resolved config and external config should match after merge:", cmp.Diff(resolvedConfig, externalConfig.Config))
	}

	// Ensure the internal config was not affected by merge
	if !cmp.Equal(clusterConfig.Config, internalConfig) {
		t.Error("Internal config should remain the same after merge:", cmp.Diff(clusterConfig.Config, internalConfig))
	}

	// Get the global config off cluster and ensure it hasn't changed
	retrievedClusterConfig := &v1alpha1.DevWorkspaceOperatorConfig{}
	namespacedName = types.NamespacedName{
		Name:      OperatorConfigName,
		Namespace: testNamespace,
	}
	err = client.Get(context.TODO(), namespacedName, retrievedClusterConfig)
	if !assert.NoError(t, err, "Should not return error when fetching config from cluster") {
		return
	}

	if !cmp.Equal(retrievedClusterConfig.Config, clusterConfig.Config) {
		t.Error("Config on cluster and global config should match after merge; global config should not have been modified from merge:", cmp.Diff(retrievedClusterConfig, clusterConfig.Config))
	}
}

func TestSetupControllerAlwaysSetsDefaultClusterRoutingSuffix(t *testing.T) {
	setupForTest(t)
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	clusterConfig := buildConfig(&v1alpha1.OperatorConfiguration{
		Routing: &v1alpha1.RoutingConfig{
			ClusterHostSuffix: "192.168.0.1.nip.io",
		},
	})
	testRoute := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      openShiftTestRouteName,
			Namespace: testNamespace,
		},
		Spec: routev1.RouteSpec{
			Host: "test-host",
		},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clusterConfig, testRoute).Build()
	err := SetupControllerConfig(client)
	if !assert.NoError(t, err, "Should not return error") {
		return
	}
	assert.Equal(t, "test-host", defaultConfig.Routing.ClusterHostSuffix, "Should set default clusterRoutingSuffix even if config overrides it initially")
	assert.Equal(t, "192.168.0.1.nip.io", internalConfig.Routing.ClusterHostSuffix, "Should use value from config for clusterRoutingSuffix")
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
	internalConfig = defaultConfig.DeepCopy()
	config := buildConfig(&v1alpha1.OperatorConfiguration{
		Workspace: &v1alpha1.WorkspaceConfig{
			ImagePullPolicy: "IfNotPresent",
		},
	})
	config.Namespace = "not-test-namespace"
	syncConfigFrom(config)
	assert.Equal(t, defaultConfig, internalConfig)
}

func TestSyncConfigFromIgnoresOtherConfigWithOtherName(t *testing.T) {
	setupForTest(t)
	internalConfig = defaultConfig.DeepCopy()
	config := buildConfig(&v1alpha1.OperatorConfiguration{
		Workspace: &v1alpha1.WorkspaceConfig{
			ImagePullPolicy: "IfNotPresent",
		},
	})
	config.Name = "testing-name"
	syncConfigFrom(config)
	assert.Equal(t, defaultConfig, internalConfig)
}

func TestSyncConfigDoesNotChangeDefaults(t *testing.T) {
	setupForTest(t)
	oldDefaultConfig := defaultConfig.DeepCopy()
	internalConfig = defaultConfig.DeepCopy()
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
	assert.Equal(t, defaultConfig, oldDefaultConfig)
}

func TestSyncConfigRestoresClusterRoutingSuffix(t *testing.T) {
	setupForTest(t)
	defaultConfig.Routing.ClusterHostSuffix = "default.routing.suffix"
	config := buildConfig(&v1alpha1.OperatorConfiguration{
		Routing: &v1alpha1.RoutingConfig{
			ClusterHostSuffix: "192.168.0.1.nip.io",
		},
	})
	syncConfigFrom(config)
	assert.Equal(t, "192.168.0.1.nip.io", internalConfig.Routing.ClusterHostSuffix, "Should update clusterRoutingSuffix from config")
	config.Config.Routing.ClusterHostSuffix = ""
	syncConfigFrom(config)
	assert.Equal(t, "default.routing.suffix", internalConfig.Routing.ClusterHostSuffix, "Should restore default clusterRoutingSuffix if it is available")
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
	assert.Equal(t, "test-host", internalConfig.Routing.ClusterHostSuffix, "Should get clusterHostSuffix from route on OpenShift")
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
	assert.Equal(t, "test-host", internalConfig.Routing.ClusterHostSuffix, "clusterHostSuffix should persist after an update")
}

func TestMergeConfigLooksAtAllFields(t *testing.T) {
	f := fuzz.New().NilChance(0).Funcs(
		func(embeddedResource *runtime.RawExtension, c fuzz.Continue) {},
	)
	expectedConfig := &v1alpha1.OperatorConfiguration{}
	actualConfig := &v1alpha1.OperatorConfiguration{}
	f.Fuzz(expectedConfig)
	mergeConfig(expectedConfig, actualConfig)
	assert.Equal(t, expectedConfig, actualConfig, "merging configs should merge all fields")
}
