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
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config/configmap"
)

func TestMigrateConfigDoesNothingWhenNoConfigMap(t *testing.T) {
	setupForTest(t)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	err := MigrateConfigFromConfigMap(client)
	assert.NoError(t, err, "Should not return error when there is no configmap")

	clusterConfig := &v1alpha1.DevWorkspaceOperatorConfig{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      OperatorConfigName,
		Namespace: testNamespace,
	}, clusterConfig)
	if assert.Error(t, err, "test client should return error when trying to get nonexistent clusterConfig") {
		assert.True(t, k8sErrors.IsNotFound(err), "expect error to be NotFound")
	}
}

func TestMigrateConfigErrorWhenConfigAndConfigMapPresent(t *testing.T) {
	setupForTest(t)
	existingConfig := buildConfig(&v1alpha1.OperatorConfiguration{
		Workspace: &v1alpha1.WorkspaceConfig{
			ImagePullPolicy: "testImagePullPolicy",
		},
	})
	existingCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmap.ConfigMapReference.Name,
			Namespace: testNamespace,
		},
		Data: map[string]string{
			"devworkspace.default_routing_class": "testRoutingClass",
		},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingCM, existingConfig).Build()
	err := MigrateConfigFromConfigMap(client)
	assert.Error(t, err, "Should return error")
	assert.Equal(t, "found both DevWorkspaceOperatorConfig and configmap on cluster -- cannot migrate", err.Error())
}

func TestMigrateConfigDeletesConfigMapWhenAlreadyMigrated(t *testing.T) {
	setupForTest(t)
	existingConfig := buildConfig(&v1alpha1.OperatorConfiguration{
		Routing: &v1alpha1.RoutingConfig{
			DefaultRoutingClass: "testRoutingClass",
		},
	})
	existingCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmap.ConfigMapReference.Name,
			Namespace: testNamespace,
		},
		Data: map[string]string{
			"devworkspace.default_routing_class": "testRoutingClass",
		},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingCM, existingConfig).Build()
	err := MigrateConfigFromConfigMap(client)
	if !assert.NoError(t, err, "Should not error") {
		return
	}
	clusterCM := &corev1.ConfigMap{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      configmap.ConfigMapReference.Name,
		Namespace: testNamespace,
	}, clusterCM)
	assert.Error(t, err, "Expect error on trying to find configmap as it is deleted")
	assert.True(t, k8sErrors.IsNotFound(err), "Expect error to be IsNotFound")
}

func TestMigrateConfigCreatesCRFromConfigMap(t *testing.T) {
	setupForTest(t)
	existingCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmap.ConfigMapReference.Name,
			Namespace: testNamespace,
		},
		Data: map[string]string{
			"devworkspace.default_routing_class":         "testRoutingClass",
			"devworkspace.routing.cluster_host_suffix":   "testHostSuffix",
			"devworkspace.sidecar.image_pull_policy":     "testImagePullPolicy",
			"devworkspace.pvc.name":                      "testPVCName",
			"devworkspace.idle_timeout":                  "testIdleTimeout",
			"devworkspace.experimental_features_enabled": "true",
		},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingCM).Build()

	expectedConfig := buildConfig(&v1alpha1.OperatorConfiguration{
		Routing: &v1alpha1.RoutingConfig{
			DefaultRoutingClass: "testRoutingClass",
			ClusterHostSuffix:   "testHostSuffix",
		},
		Workspace: &v1alpha1.WorkspaceConfig{
			ImagePullPolicy: "testImagePullPolicy",
			PVCName:         "testPVCName",
			IdleTimeout:     "testIdleTimeout",
		},
		EnableExperimentalFeatures: pointer.Bool(true),
	})

	err := MigrateConfigFromConfigMap(client)
	if !assert.NoError(t, err, "Should not error") {
		return
	}
	clusterConfig := &v1alpha1.DevWorkspaceOperatorConfig{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      OperatorConfigName,
		Namespace: testNamespace,
	}, clusterConfig)
	if !assert.NoError(t, err, "Should create config CRD on cluster from configmap") {
		return
	}
	assert.Equal(t, expectedConfig.Config, clusterConfig.Config, "Expect configmap to be converted to config CRD")
	clusterCM := &corev1.ConfigMap{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      configmap.ConfigMapReference.Name,
		Namespace: testNamespace,
	}, clusterCM)
	assert.Error(t, err, "Expect error on trying to find configmap as it is deleted")
	assert.True(t, k8sErrors.IsNotFound(err), "Expect error to be IsNotFound")
}

func TestMigrateConfigSucceedsWhenCRDHasAllValuesFromConfigMap(t *testing.T) {
	setupForTest(t)
	existingCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmap.ConfigMapReference.Name,
			Namespace: testNamespace,
		},
		Data: map[string]string{
			"devworkspace.default_routing_class":       "testRoutingClass",
			"devworkspace.routing.cluster_host_suffix": "testHostSuffix",
		},
	}
	existingConfig := buildConfig(&v1alpha1.OperatorConfiguration{
		Routing: &v1alpha1.RoutingConfig{
			DefaultRoutingClass: "testRoutingClass",
			ClusterHostSuffix:   "testHostSuffix",
		},
		Workspace: &v1alpha1.WorkspaceConfig{
			ImagePullPolicy: "testImagePullPolicy",
			PVCName:         "testPVCName",
			IdleTimeout:     "testIdleTimeout",
		},
	})

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingConfig, existingCM).Build()

	expectedConfig := existingConfig.DeepCopy()

	err := MigrateConfigFromConfigMap(client)
	if !assert.NoError(t, err, "Should not error") {
		return
	}
	clusterConfig := &v1alpha1.DevWorkspaceOperatorConfig{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      OperatorConfigName,
		Namespace: testNamespace,
	}, clusterConfig)
	if !assert.NoError(t, err, "Config CRD should exist on cluster after successful migration") {
		return
	}
	assert.Equal(t, expectedConfig.Config, clusterConfig.Config, "Expect config CRD to be unchanged")
	clusterCM := &corev1.ConfigMap{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      configmap.ConfigMapReference.Name,
		Namespace: testNamespace,
	}, clusterCM)
	assert.Error(t, err, "Expect error on trying to find configmap as it is deleted")
	assert.True(t, k8sErrors.IsNotFound(err), "Expect error to be IsNotFound")
}

func TestConvertConfigMapDoesNothingWhenNoConfigmap(t *testing.T) {
	setupForTest(t)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	migratedConfig, err := convertConfigMapToConfigCRD(client)
	assert.NoError(t, err, "Should not return error when there is no configmap")
	assert.Nil(t, migratedConfig, "Should not create migrated config object when there is no configmap")
}

func TestConvertConfigMapGetsAllOldConfigValues(t *testing.T) {
	setupForTest(t)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmap.ConfigMapReference.Name,
			Namespace: testNamespace,
		},
		Data: map[string]string{
			"devworkspace.default_routing_class":         "testRoutingClass",
			"devworkspace.routing.cluster_host_suffix":   "testHostSuffix",
			"devworkspace.sidecar.image_pull_policy":     "testImagePullPolicy",
			"devworkspace.pvc.name":                      "testPVCName",
			"devworkspace.pvc.storage_class.name":        "testStorageClassName",
			"devworkspace.idle_timeout":                  "testIdleTimeout",
			"devworkspace.experimental_features_enabled": "true",
		},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	testStorageClassName := "testStorageClassName"
	expectedConfig := buildConfig(&v1alpha1.OperatorConfiguration{
		Routing: &v1alpha1.RoutingConfig{
			DefaultRoutingClass: "testRoutingClass",
			ClusterHostSuffix:   "testHostSuffix",
		},
		Workspace: &v1alpha1.WorkspaceConfig{
			ImagePullPolicy:  "testImagePullPolicy",
			PVCName:          "testPVCName",
			StorageClassName: &testStorageClassName,
			IdleTimeout:      "testIdleTimeout",
		},
		EnableExperimentalFeatures: pointer.Bool(true),
	})

	migratedConfig, err := convertConfigMapToConfigCRD(client)
	if !assert.NoError(t, err, "Should not return error when there is no configmap") {
		return
	}
	assert.Equal(t, expectedConfig, migratedConfig, "Should pick up all values in config")
}

func TestConvertConfigMapIgnoresValuesThatAreDefault(t *testing.T) {
	setupForTest(t)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmap.ConfigMapReference.Name,
			Namespace: testNamespace,
		},
		Data: map[string]string{
			"devworkspace.default_routing_class":         "basic",
			"devworkspace.routing.cluster_host_suffix":   "testHostSuffix",
			"devworkspace.sidecar.image_pull_policy":     "Always",
			"devworkspace.pvc.name":                      "testPVCName",
			"devworkspace.idle_timeout":                  "testIdleTimeout",
			"devworkspace.experimental_features_enabled": "false",
		},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	expectedConfig := buildConfig(&v1alpha1.OperatorConfiguration{
		Routing: &v1alpha1.RoutingConfig{
			ClusterHostSuffix: "testHostSuffix",
		},
		Workspace: &v1alpha1.WorkspaceConfig{
			PVCName:     "testPVCName",
			IdleTimeout: "testIdleTimeout",
		},
	})

	migratedConfig, err := convertConfigMapToConfigCRD(client)
	if !assert.NoError(t, err, "Should not return error when there is no configmap") {
		return
	}
	assert.Equal(t, expectedConfig, migratedConfig, "Should drop default values in configmap")
}

func TestConvertConfigMapReturnsNilWhenAllDefault(t *testing.T) {
	setupForTest(t)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmap.ConfigMapReference.Name,
			Namespace: testNamespace,
		},
		Data: map[string]string{
			"devworkspace.default_routing_class":         "basic",
			"devworkspace.sidecar.image_pull_policy":     "Always",
			"devworkspace.pvc.name":                      "claim-devworkspace",
			"devworkspace.idle_timeout":                  "15m",
			"devworkspace.experimental_features_enabled": "false",
		},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	migratedConfig, err := convertConfigMapToConfigCRD(client)
	if !assert.NoError(t, err, "Should not return error when there is no configmap") {
		return
	}
	assert.Nil(t, migratedConfig, "Should return (nil, nil) when configmap is all default")
}
