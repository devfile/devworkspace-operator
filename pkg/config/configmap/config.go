//
// Copyright (c) 2019-2023 Red Hat, Inc.
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

package configmap

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
)

var ControllerCfg ControllerConfig
var log = logf.Log.WithName("controller_devworkspace_config")

const (
	ConfigMapNameEnvVar      = "CONTROLLER_CONFIG_MAP_NAME"
	ConfigMapNamespaceEnvVar = "CONTROLLER_CONFIG_MAP_NAMESPACE"
)

var ConfigMapReference = client.ObjectKey{
	Namespace: "",
	Name:      "devworkspace-controller-configmap",
}

type ControllerConfig struct {
	configMap *corev1.ConfigMap
}

func (wc *ControllerConfig) update(configMap *corev1.ConfigMap) {
	log.Info("Updating the configuration from config map '%s' in namespace '%s'", configMap.Name, configMap.Namespace)
	wc.configMap = configMap
}

func (wc *ControllerConfig) GetWorkspacePVCName() *string {
	return wc.GetProperty(workspacePVCName)
}

func (wc *ControllerConfig) GetDefaultRoutingClass() *string {
	return wc.GetProperty(routingClass)
}

func (wc *ControllerConfig) GetClusterRoutingSuffix() *string {
	return wc.GetProperty(routingSuffix)
}

// GetExperimentalFeaturesEnabled returns true if experimental features should be enabled.
// DO NOT TURN ON IT IN THE PRODUCTION.
// Experimental features are not well tested and may be totally removed without announcement.
func (wc *ControllerConfig) GetExperimentalFeaturesEnabled() *string {
	return wc.GetProperty(experimentalFeaturesEnabled)
}

func (wc *ControllerConfig) GetPVCStorageClassName() *string {
	return wc.GetProperty(workspacePVCStorageClassName)
}

func (wc *ControllerConfig) GetSidecarPullPolicy() *string {
	return wc.GetProperty(sidecarPullPolicy)
}

func (wc *ControllerConfig) GetProperty(name string) *string {
	val, exists := wc.configMap.Data[name]
	if exists {
		return &val
	}
	return nil
}

func (wc *ControllerConfig) Validate() error {
	return nil
}

func (wc *ControllerConfig) GetWorkspaceIdleTimeout() *string {
	return wc.GetProperty(devworkspaceIdleTimeout)
}

func syncConfigmapFromCluster(client client.Client, obj client.Object) {
	if obj.GetNamespace() != ConfigMapReference.Namespace ||
		obj.GetName() != ConfigMapReference.Name {
		return
	}
	if cm, isConfigMap := obj.(*corev1.ConfigMap); isConfigMap {
		ControllerCfg.update(cm)
		return
	}
}

func LoadControllerConfig(nonCachedClient client.Client) (found bool, err error) {
	configMapName, found := os.LookupEnv(ConfigMapNameEnvVar)
	if found && len(configMapName) > 0 {
		ConfigMapReference.Name = configMapName
	}
	configMapNamespace, found := os.LookupEnv(ConfigMapNamespaceEnvVar)
	if found && len(configMapNamespace) > 0 {
		ConfigMapReference.Namespace = configMapNamespace
	} else {
		namespace, err := infrastructure.GetNamespace()
		if err != nil {
			return false, err
		}
		ConfigMapReference.Namespace = namespace
	}

	if ConfigMapReference.Namespace == "" {
		return false, fmt.Errorf("you should set the namespace of the controller config map through the '%s' environment variable", ConfigMapNamespaceEnvVar)
	}

	configMap := &corev1.ConfigMap{}
	log.Info(fmt.Sprintf("Searching for config map '%s' in namespace '%s'", ConfigMapReference.Name, ConfigMapReference.Namespace))
	err = nonCachedClient.Get(context.TODO(), ConfigMapReference, configMap)
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			return false, err
		}
		return false, nil
	} else {
		log.Info(fmt.Sprintf("  => found config map '%s' in namespace '%s'", configMap.GetObjectMeta().GetName(), configMap.GetObjectMeta().GetNamespace()))
	}

	if configMap.Data == nil {
		configMap.Data = map[string]string{}
	}
	syncConfigmapFromCluster(nonCachedClient, configMap)

	return true, nil
}
