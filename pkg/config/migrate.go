//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package config

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	dw "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config/configmap"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
)

func MigrateConfigFromConfigMap(client crclient.Client) error {
	migratedConfig, err := convertConfigMapToConfigCRD(client)
	if err != nil {
		return err
	}
	if migratedConfig == nil {
		return nil
	}

	namespace, err := infrastructure.GetNamespace()
	if err != nil {
		return err
	}
	clusterConfig, err := getClusterConfig(namespace, client)
	if err != nil {
		return err
	}
	if clusterConfig != nil {
		// Check using DeepDerivative in case cluster config contains default/additional values -- we only care
		// that values in migratedConfig are propagated to the cluster DWOC.
		if equality.Semantic.DeepDerivative(migratedConfig.Config, clusterConfig.Config) {
			log.Info("Found deprecated operator configmap matching config custom resource. Deleting.")
			// In case we migrated before but failed to delete
			return deleteMigratedConfigmap(client)
		}
		return fmt.Errorf("found both DevWorkspaceOperatorConfig and configmap on cluster -- cannot migrate")
	}

	// Set namespace in case obsolete env vars were used to specify a custom namespace for the configmap
	migratedConfig.Namespace = namespace
	if err := client.Create(context.Background(), migratedConfig); err != nil {
		return err
	}
	log.Info("Migrated operator configuration from configmap")
	return deleteMigratedConfigmap(client)
}

func deleteMigratedConfigmap(client crclient.Client) error {
	obsoleteConfigmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmap.ConfigMapReference.Name,
			Namespace: configmap.ConfigMapReference.Namespace,
		},
	}
	return client.Delete(context.Background(), obsoleteConfigmap)
}

// convertConfigMapToConfigCRD converts a earlier devworkspace configuration configmap (if present)
// into a DevWorkspaceOperatorConfig. Values matching the current default config settings are ignored.
// If the configmap is not present, or if the configmap is present but all values are default, returns
// nil. Returns an error if we fail to load the controller config from configmap.
func convertConfigMapToConfigCRD(client crclient.Client) (*dw.DevWorkspaceOperatorConfig, error) {
	found, err := configmap.LoadControllerConfig(client)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}

	migratedRoutingConfig := &dw.RoutingConfig{}
	setRoutingConfig := false
	routingSuffix := configmap.ControllerCfg.GetClusterRoutingSuffix()
	if routingSuffix != nil && *routingSuffix != DefaultConfig.Routing.ClusterHostSuffix {
		migratedRoutingConfig.ClusterHostSuffix = *routingSuffix
		setRoutingConfig = true
	}
	defaultRoutingClass := configmap.ControllerCfg.GetDefaultRoutingClass()
	if defaultRoutingClass != nil && *defaultRoutingClass != DefaultConfig.Routing.DefaultRoutingClass {
		migratedRoutingConfig.DefaultRoutingClass = *defaultRoutingClass
		setRoutingConfig = true
	}

	migratedWorkspaceConfig := &dw.WorkspaceConfig{}
	setWorkspaceConfig := false
	storageClassName := configmap.ControllerCfg.GetPVCStorageClassName()
	if storageClassName != DefaultConfig.Workspace.StorageClassName {
		migratedWorkspaceConfig.StorageClassName = storageClassName
		setWorkspaceConfig = true
	}
	sidecarPullPolicy := configmap.ControllerCfg.GetSidecarPullPolicy()
	if sidecarPullPolicy != nil && *sidecarPullPolicy != DefaultConfig.Workspace.ImagePullPolicy {
		migratedWorkspaceConfig.ImagePullPolicy = *sidecarPullPolicy
		setWorkspaceConfig = true
	}
	idleTimeout := configmap.ControllerCfg.GetWorkspaceIdleTimeout()
	if idleTimeout != nil && *idleTimeout != DefaultConfig.Workspace.IdleTimeout {
		migratedWorkspaceConfig.IdleTimeout = *idleTimeout
		setWorkspaceConfig = true
	}
	pvcName := configmap.ControllerCfg.GetWorkspacePVCName()
	if pvcName != nil && *pvcName != DefaultConfig.Workspace.PVCName {
		migratedWorkspaceConfig.PVCName = *pvcName
		setWorkspaceConfig = true
	}

	var experimentalFeatures *bool
	experimentalFeaturesStr := configmap.ControllerCfg.GetExperimentalFeaturesEnabled()
	if experimentalFeaturesStr != nil && *experimentalFeaturesStr == "true" {
		trueBool := true
		experimentalFeatures = &trueBool
	}

	if !setRoutingConfig && !setWorkspaceConfig && experimentalFeatures == nil {
		return nil, nil
	}

	migratedConfig := &dw.DevWorkspaceOperatorConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OperatorConfigName,
			Namespace: configmap.ConfigMapReference.Namespace,
		},
		Config: &dw.OperatorConfiguration{},
	}
	migratedConfig.Config.EnableExperimentalFeatures = experimentalFeatures
	if setRoutingConfig {
		migratedConfig.Config.Routing = migratedRoutingConfig
	}
	if setWorkspaceConfig {
		migratedConfig.Config.Workspace = migratedWorkspaceConfig
	}
	return migratedConfig, nil
}
