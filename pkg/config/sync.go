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
	"sync"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	controller "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
)

const OperatorConfigName = "devworkspace-operator-config"

var internalConfig *controller.OperatorConfiguration
var configMutex sync.Mutex
var Routing *controller.RoutingConfig
var Workspace *controller.WorkspaceConfig
var configNamespace string

func SetConfigForTesting(config *controller.OperatorConfiguration) {
	configMutex.Lock()
	defer configMutex.Unlock()
	internalConfig = config.DeepCopy()
	updatePublicConfig()
}

func SetupControllerConfig(client crclient.Client) error {
	if internalConfig != nil {
		return fmt.Errorf("internal controller configuration is already set up")
	}
	internalConfig = &controller.OperatorConfiguration{}
	namespace, err := infrastructure.GetNamespace()
	if err != nil {
		return err
	}
	configNamespace = namespace
	config, err := getClusterConfig(configNamespace, client)
	if err != nil {
		return err
	}
	if config == nil {
		internalConfig = DefaultConfig.DeepCopy()
		updatePublicConfig()
	} else {
		syncConfigFrom(config)
	}
	return nil
}

func ExperimentalFeaturesEnabled() bool {
	if internalConfig.EnableExperimentalFeatures == nil {
		return false
	}
	return *internalConfig.EnableExperimentalFeatures
}

func getClusterConfig(namespace string, client crclient.Client) (*controller.DevWorkspaceOperatorConfig, error) {
	clusterConfig := &controller.DevWorkspaceOperatorConfig{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: OperatorConfigName, Namespace: namespace}, clusterConfig); err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return clusterConfig, nil
}

func syncConfigFrom(newConfig *controller.DevWorkspaceOperatorConfig) {
	if newConfig == nil || newConfig.Name != OperatorConfigName || newConfig.Namespace != configNamespace {
		return
	}
	configMutex.Lock()
	defer configMutex.Unlock()
	internalConfig = DefaultConfig.DeepCopy()
	mergeConfig(newConfig.Config, internalConfig)
	updatePublicConfig()
}

func restoreDefaultConfig() {
	configMutex.Lock()
	defer configMutex.Unlock()
	internalConfig = DefaultConfig.DeepCopy()
	updatePublicConfig()
}

func updatePublicConfig() {
	Routing = internalConfig.Routing.DeepCopy()
	Workspace = internalConfig.Workspace.DeepCopy()
}

func mergeConfig(from, to *controller.OperatorConfiguration) {
	if to == nil {
		to = &controller.OperatorConfiguration{}
	}
	if from == nil {
		return
	}
	if from.EnableExperimentalFeatures != nil {
		to.EnableExperimentalFeatures = from.EnableExperimentalFeatures
	}
	if from.Routing != nil {
		if to.Routing == nil {
			to.Routing = &controller.RoutingConfig{}
		}
		if from.Routing.DefaultRoutingClass != "" {
			to.Routing.DefaultRoutingClass = from.Routing.DefaultRoutingClass
		}
		if from.Routing.ClusterHostSuffix != "" {
			to.Routing.ClusterHostSuffix = from.Routing.ClusterHostSuffix
		}
	}
	if from.Workspace != nil {
		if to.Workspace == nil {
			to.Workspace = &controller.WorkspaceConfig{}
		}
		if from.Workspace.StorageClassName != nil {
			to.Workspace.StorageClassName = from.Workspace.StorageClassName
		}
		if from.Workspace.PVCName != "" {
			to.Workspace.PVCName = from.Workspace.PVCName
		}
		if from.Workspace.ImagePullPolicy != "" {
			to.Workspace.ImagePullPolicy = from.Workspace.ImagePullPolicy
		}
		if from.Workspace.IdleTimeout != "" {
			to.Workspace.IdleTimeout = from.Workspace.IdleTimeout
		}
		if from.Workspace.IgnoredUnrecoverableEvents != nil {
			to.Workspace.IgnoredUnrecoverableEvents = from.Workspace.IgnoredUnrecoverableEvents
		}
	}
}
