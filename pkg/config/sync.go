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
	"strings"
	"sync"

	"github.com/devfile/devworkspace-operator/pkg/config/proxy"
	routeV1 "github.com/openshift/api/route/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	controller "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
)

const (
	OperatorConfigName     = "devworkspace-operator-config"
	openShiftTestRouteName = "devworkspace-controller-test-route"
)

var (
	Routing         *controller.RoutingConfig
	Workspace       *controller.WorkspaceConfig
	InternalConfig  *controller.OperatorConfiguration // TODO: Make this unexported again
	configMutex     sync.Mutex
	configNamespace string
	log             = ctrl.Log.WithName("operator-configuration")
)

// TODO: Refactor this to return the modified config, so test classes don't acces the InternalConfig
func SetConfigForTesting(config *controller.OperatorConfiguration) {
	configMutex.Lock()
	defer configMutex.Unlock()
	InternalConfig = defaultConfig.DeepCopy()
	mergeConfig(config, InternalConfig)
	updatePublicConfig()
}

func SetupControllerConfig(client crclient.Client) error {
	if InternalConfig != nil {
		return fmt.Errorf("internal controller configuration is already set up")
	}
	InternalConfig = &controller.OperatorConfiguration{}

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
		InternalConfig = defaultConfig.DeepCopy()
	} else {
		syncConfigFrom(config)
	}

	defaultRoutingSuffix, err := discoverRouteSuffix(client)
	if err != nil {
		return err
	}
	defaultConfig.Routing.ClusterHostSuffix = defaultRoutingSuffix
	if InternalConfig.Routing.ClusterHostSuffix == "" {
		InternalConfig.Routing.ClusterHostSuffix = defaultRoutingSuffix
	}

	clusterProxy, err := proxy.GetClusterProxyConfig(client)
	if err != nil {
		return err
	}
	defaultConfig.Routing.ProxyConfig = clusterProxy
	InternalConfig.Routing.ProxyConfig = proxy.MergeProxyConfigs(clusterProxy, InternalConfig.Routing.ProxyConfig)

	updatePublicConfig()
	return nil
}

func IsSetUp() bool {
	return InternalConfig != nil
}

func ExperimentalFeaturesEnabled() bool {
	if InternalConfig == nil || InternalConfig.EnableExperimentalFeatures == nil {
		return false
	}
	return *InternalConfig.EnableExperimentalFeatures
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
	InternalConfig = defaultConfig.DeepCopy()
	mergeConfig(newConfig.Config, InternalConfig)
	updatePublicConfig()
}

func restoreDefaultConfig() {
	configMutex.Lock()
	defer configMutex.Unlock()
	InternalConfig = defaultConfig.DeepCopy()
	updatePublicConfig()
}

func updatePublicConfig() {
	Routing = InternalConfig.Routing.DeepCopy()
	Workspace = InternalConfig.Workspace.DeepCopy()
	logCurrentConfig()
}

// discoverRouteSuffix attempts to determine a clusterHostSuffix that is compatible with the current cluster.
// On OpenShift, this is done by creating a temporary route and reading the auto-filled .spec.host. On Kubernetes,
// there's no way to determine this value automatically so ("", nil) is returned.
func discoverRouteSuffix(client crclient.Client) (string, error) {
	if !infrastructure.IsOpenShift() {
		return "", nil
	}

	testRoute := &routeV1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: configNamespace,
			Name:      openShiftTestRouteName,
		},
		Spec: routeV1.RouteSpec{
			To: routeV1.RouteTargetReference{
				Kind: "Service",
				Name: "devworkspace-controller-test-route",
			},
		},
	}

	err := client.Create(context.TODO(), testRoute)
	if err != nil {
		if k8sErrors.IsAlreadyExists(err) {
			err := client.Get(context.TODO(), types.NamespacedName{
				Name:      openShiftTestRouteName,
				Namespace: configNamespace,
			}, testRoute)
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}
	defer client.Delete(context.TODO(), testRoute)
	host := testRoute.Spec.Host
	prefixToRemove := fmt.Sprintf("%s-%s.", openShiftTestRouteName, configNamespace)
	host = strings.TrimPrefix(host, prefixToRemove)
	return host, nil
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
		if from.Routing.ProxyConfig != nil {
			if to.Routing.ProxyConfig == nil {
				to.Routing.ProxyConfig = &controller.Proxy{}
			}
			to.Routing.ProxyConfig = proxy.MergeProxyConfigs(from.Routing.ProxyConfig, defaultConfig.Routing.ProxyConfig)
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
		if from.Workspace.ProgressTimeout != "" {
			to.Workspace.ProgressTimeout = from.Workspace.ProgressTimeout
		}
		if from.Workspace.IgnoredUnrecoverableEvents != nil {
			to.Workspace.IgnoredUnrecoverableEvents = from.Workspace.IgnoredUnrecoverableEvents
		}
		if from.Workspace.CleanupOnStop != nil {
			to.Workspace.CleanupOnStop = from.Workspace.CleanupOnStop
		}
		if from.Workspace.PodSecurityContext != nil {
			to.Workspace.PodSecurityContext = from.Workspace.PodSecurityContext
		}
		if from.Workspace.DefaultStorageSize != nil {
			if to.Workspace.DefaultStorageSize == nil {
				to.Workspace.DefaultStorageSize = &controller.StorageSizes{}
			}
			if from.Workspace.DefaultStorageSize.Common != nil {
				commonSizeCopy := from.Workspace.DefaultStorageSize.Common.DeepCopy()
				to.Workspace.DefaultStorageSize.Common = &commonSizeCopy
			}
			if from.Workspace.DefaultStorageSize.PerWorkspace != nil {
				perWorkspaceSizeCopy := from.Workspace.DefaultStorageSize.PerWorkspace.DeepCopy()
				to.Workspace.DefaultStorageSize.PerWorkspace = &perWorkspaceSizeCopy
			}
		}
		if from.Workspace.DefaultTemplate != nil {
			templateSpecContentCopy := from.Workspace.DefaultTemplate.DeepCopy()
			to.Workspace.DefaultTemplate = templateSpecContentCopy
		}
	}
}

// logCurrentConfig formats the current operator configuration as a plain string
func logCurrentConfig() {
	if InternalConfig == nil {
		return
	}
	var config []string
	if Routing != nil {
		if Routing.ClusterHostSuffix != "" && Routing.ClusterHostSuffix != defaultConfig.Routing.ClusterHostSuffix {
			config = append(config, fmt.Sprintf("routing.clusterHostSuffix=%s", Routing.ClusterHostSuffix))
		}
		if Routing.DefaultRoutingClass != defaultConfig.Routing.DefaultRoutingClass {
			config = append(config, fmt.Sprintf("routing.defaultRoutingClass=%s", Routing.DefaultRoutingClass))
		}
	}
	if Workspace != nil {
		if Workspace.ImagePullPolicy != defaultConfig.Workspace.ImagePullPolicy {
			config = append(config, fmt.Sprintf("workspace.imagePullPolicy=%s", Workspace.ImagePullPolicy))
		}
		if Workspace.PVCName != defaultConfig.Workspace.PVCName {
			config = append(config, fmt.Sprintf("workspace.pvcName=%s", Workspace.PVCName))
		}
		if Workspace.StorageClassName != nil && Workspace.StorageClassName != defaultConfig.Workspace.StorageClassName {
			config = append(config, fmt.Sprintf("workspace.storageClassName=%s", *Workspace.StorageClassName))
		}
		if Workspace.IdleTimeout != defaultConfig.Workspace.IdleTimeout {
			config = append(config, fmt.Sprintf("workspace.idleTimeout=%s", Workspace.IdleTimeout))
		}
		if Workspace.IgnoredUnrecoverableEvents != nil {
			config = append(config, fmt.Sprintf("workspace.ignoredUnrecoverableEvents=%s",
				strings.Join(Workspace.IgnoredUnrecoverableEvents, ";")))
		}
		if Workspace.DefaultStorageSize != nil {
			if Workspace.DefaultStorageSize.Common != nil {
				config = append(config, fmt.Sprintf("workspace.defaultStorageSize.common=%s", Workspace.DefaultStorageSize.Common.String()))
			}
			if Workspace.DefaultStorageSize.PerWorkspace != nil {
				config = append(config, fmt.Sprintf("workspace.defaultStorageSize.perWorkspace=%s", Workspace.DefaultStorageSize.PerWorkspace.String()))
			}
		}
		if Workspace.DefaultTemplate != nil {
			config = append(config, fmt.Sprintf("workspace.defaultTemplate is set"))
		}
	}
	if InternalConfig.EnableExperimentalFeatures != nil && *InternalConfig.EnableExperimentalFeatures {
		config = append(config, "enableExperimentalFeatures=true")
	}

	if len(config) == 0 {
		log.Info("Updated config to [(default config)]")
	} else {
		log.Info(fmt.Sprintf("Updated config to [%s]", strings.Join(config, ",")))
	}

	if InternalConfig.Routing.ProxyConfig != nil {
		log.Info("Resolved proxy configuration", "proxy", InternalConfig.Routing.ProxyConfig)
	}
}
