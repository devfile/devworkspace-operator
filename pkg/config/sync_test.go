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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
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
		EnableExperimentalFeatures: pointer.Bool(true),
	})
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clusterConfig).Build()

	expectedConfig := defaultConfig.DeepCopy()
	expectedConfig.Routing.DefaultRoutingClass = "test-routingClass"
	expectedConfig.Routing.ClusterHostSuffix = "192.168.0.1.nip.io"
	expectedConfig.Workspace.ImagePullPolicy = "IfNotPresent"
	expectedConfig.EnableExperimentalFeatures = pointer.Bool(true)

	err := SetupControllerConfig(client)
	if !assert.NoError(t, err, "Should not return error") {
		return
	}
	assert.Equal(t, expectedConfig, internalConfig, fmt.Sprintf("Processed config should merge settings from cluster: %s", cmp.Diff(internalConfig, expectedConfig)))
}

func TestMergesAllFieldsFromClusterConfig(t *testing.T) {
	setupForTest(t)
	f := fuzz.New().NilChance(0).Funcs(
		func(_ *v1alpha1.StorageSizes, c fuzz.Continue) {},
		func(_ *dw.DevWorkspaceTemplateSpecContent, c fuzz.Continue) {},
		// Ensure no empty strings are generated as they cause default values to be used
		func(s *string, c fuzz.Continue) { *s = "a" + c.RandString() },
		// The only valid deployment strategies are Recreate and RollingUpdate
		func(deploymentStrategy *appsv1.DeploymentStrategyType, c fuzz.Continue) {
			if c.Int()%2 == 0 {
				*deploymentStrategy = appsv1.RollingUpdateDeploymentStrategyType
			} else {
				*deploymentStrategy = appsv1.RecreateDeploymentStrategyType
			}
		},
		fuzzQuantity,
		fuzzResourceList,
		fuzzResourceRequirements,
	)
	for i := 0; i < 100; i++ {
		fuzzedConfig := &v1alpha1.OperatorConfiguration{}
		f.Fuzz(fuzzedConfig)
		// Skip checking these two fields as they're interface fields and hard to fuzz.
		fuzzedConfig.Workspace.DefaultStorageSize = defaultConfig.Workspace.DefaultStorageSize.DeepCopy()
		fuzzedConfig.Workspace.PodSecurityContext = defaultConfig.Workspace.PodSecurityContext.DeepCopy()
		clusterConfig := buildConfig(fuzzedConfig)
		client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clusterConfig).Build()
		err := SetupControllerConfig(client)
		if !assert.NoError(t, err, "Should not return error") {
			return
		}
		assert.Equal(t, fuzzedConfig, internalConfig, fmt.Sprintf("Processed config should merge all fields: %s", cmp.Diff(internalConfig, fuzzedConfig)))
		internalConfig = nil
	}
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
		EnableExperimentalFeatures: pointer.Bool(true),
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

func TestMergeConfigHandlesProxySettings(t *testing.T) {
	setupForTest(t)
	baseProxyConfig := &v1alpha1.Proxy{
		HttpProxy:  pointer.String("baseHttpProxy"),
		HttpsProxy: pointer.String("baseHttpsProxy"),
		NoProxy:    pointer.String("baseNoProxy"),
	}
	defaultConfig.Routing.ProxyConfig = baseProxyConfig

	tests := []struct {
		name     string
		message  string
		input    *v1alpha1.Proxy
		expected *v1alpha1.Proxy
	}{
		{
			name:    "Merges non-empty proxy settings",
			message: "Non-empty fields in proxy should be merged",
			input: &v1alpha1.Proxy{
				HttpProxy:  pointer.String("testHttpProxy"),
				HttpsProxy: pointer.String("testHttpsProxy"),
				NoProxy:    pointer.String("testNoProxy"),
			},
			expected: &v1alpha1.Proxy{
				HttpProxy:  pointer.String("testHttpProxy"),
				HttpsProxy: pointer.String("testHttpsProxy"),
				NoProxy:    pointer.String("baseNoProxy,testNoProxy"),
			},
		},
		{
			name:    "Empty string unsets proxy fields",
			message: "Merging an empty string should delete the corresponding proxy field",
			input: &v1alpha1.Proxy{
				HttpProxy:  pointer.String(""),
				HttpsProxy: pointer.String(""),
				NoProxy:    pointer.String(""),
			},
			expected: &v1alpha1.Proxy{},
		},
		{
			name:    "Using nil for fields leaves base unchanged",
			message: "Merging an empty string should delete the corresponding proxy field",
			input: &v1alpha1.Proxy{
				HttpProxy:  nil,
				HttpsProxy: nil,
				NoProxy:    nil,
			},
			expected: baseProxyConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fromConfig := &v1alpha1.OperatorConfiguration{
				Routing: &v1alpha1.RoutingConfig{
					ProxyConfig: tt.input,
				},
			}
			actualConfig := &v1alpha1.OperatorConfiguration{
				Routing: &v1alpha1.RoutingConfig{
					ProxyConfig: baseProxyConfig,
				},
			}
			mergeConfig(fromConfig, actualConfig)
			assert.Equal(t, tt.expected, actualConfig.Routing.ProxyConfig, tt.message)
		})
	}
}

func TestMergeConfigLooksAtAllFields(t *testing.T) {
	f := fuzz.New().NilChance(0).Funcs(
		func(embeddedResource *runtime.RawExtension, c fuzz.Continue) {},
		fuzzQuantity,
		fuzzResourceList,
		fuzzResourceRequirements,
		fuzzStringPtr,
	)
	expectedConfig := &v1alpha1.OperatorConfiguration{}
	actualConfig := &v1alpha1.OperatorConfiguration{}
	f.Fuzz(expectedConfig)
	mergeConfig(expectedConfig, actualConfig)
	assert.Equal(t, expectedConfig, actualConfig, "merging configs should merge all fields")
}

func fuzzQuantity(q *resource.Quantity, c fuzz.Continue) {
	q.Set(c.Int63n(999))
	q.Format = resource.DecimalSI
	_ = q.String()
}

func fuzzResourceList(resourceList *corev1.ResourceList, c fuzz.Continue) {
	memReq := resource.Quantity{}
	c.Fuzz(&memReq)
	cpuReq := resource.Quantity{}
	c.Fuzz(&cpuReq)
	*resourceList = corev1.ResourceList{
		corev1.ResourceMemory: memReq,
		corev1.ResourceCPU:    cpuReq,
	}
}

func fuzzResourceRequirements(req *corev1.ResourceRequirements, c fuzz.Continue) {
	limits, requests := corev1.ResourceList{}, corev1.ResourceList{}
	c.Fuzz(&limits)
	c.Fuzz(&requests)
	req.Limits = limits
	req.Requests = requests
}

func fuzzStringPtr(str *string, c fuzz.Continue) {
	randString := c.RandString()
	// Only set string pointer if the generated string is not empty to avoid edge cases with
	// replacing empty strings with nils in sync code. This edge case has to be tested manually.
	if randString != "" {
		*str = randString
	}
}
