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

package config

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/config/proxy"
	routeV1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	controller "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
)

const (
	OperatorConfigName     = "devworkspace-operator-config"
	openShiftTestRouteName = "devworkspace-controller-test-route"
)

var (
	internalConfig  *controller.OperatorConfiguration
	configMutex     sync.Mutex
	configNamespace string
	log             = ctrl.Log.WithName("operator-configuration")
)

func GetGlobalConfig() *controller.OperatorConfiguration {
	return internalConfig.DeepCopy()
}

// ResolveConfigForWorkspace returns the resulting config from merging the global DevWorkspaceOperatorConfig with the
// DevWorkspaceOperatorConfig specified by the optional workspace attribute `controller.devfile.io/devworkspace-config`.
// If the `controller.devfile.io/devworkspace-config` is not set, the global DevWorkspaceOperatorConfig is returned.
// If the `controller.devfile.io/devworkspace-config` attribute is incorrectly set, or the specified DevWorkspaceOperatorConfig
// does not exist on the cluster, an error is returned.
func ResolveConfigForWorkspace(workspace *dw.DevWorkspace, client crclient.Client) (*controller.OperatorConfiguration, error) {
	if !workspace.Spec.Template.Attributes.Exists(constants.ExternalDevWorkspaceConfiguration) {
		return GetGlobalConfig(), nil
	}

	namespacedName := types.NamespacedName{}
	err := workspace.Spec.Template.Attributes.GetInto(constants.ExternalDevWorkspaceConfiguration, &namespacedName)
	if err != nil {
		return nil, fmt.Errorf("failed to read attribute %s in DevWorkspace attributes: %w", constants.ExternalDevWorkspaceConfiguration, err)
	}

	if namespacedName.Name == "" {
		return nil, fmt.Errorf("'name' must be set for attribute %s in DevWorkspace attributes", constants.ExternalDevWorkspaceConfiguration)
	}

	if namespacedName.Namespace == "" {
		return nil, fmt.Errorf("'namespace' must be set for attribute %s in DevWorkspace attributes", constants.ExternalDevWorkspaceConfiguration)
	}

	externalDWOC := &controller.DevWorkspaceOperatorConfig{}
	err = client.Get(context.TODO(), namespacedName, externalDWOC)
	if err != nil {
		return nil, fmt.Errorf("could not fetch external DWOC with name %s in namespace %s: %w", namespacedName.Name, namespacedName.Namespace, err)
	}
	return getMergedConfig(externalDWOC.Config, internalConfig), nil
}

func GetConfigForTesting(customConfig *controller.OperatorConfiguration) *controller.OperatorConfiguration {
	configMutex.Lock()
	defer configMutex.Unlock()
	testConfig := defaultConfig.DeepCopy()
	mergeConfig(customConfig, testConfig)
	return testConfig
}

func SetGlobalConfigForTesting(testConfig *controller.OperatorConfiguration) {
	configMutex.Lock()
	defer configMutex.Unlock()
	setDefaultPodSecurityContext()
	internalConfig = defaultConfig.DeepCopy()
	mergeConfig(testConfig, internalConfig)
}

func SetupControllerConfig(client crclient.Client) error {
	if internalConfig != nil {
		return fmt.Errorf("internal controller configuration is already set up")
	}
	if err := setDefaultPodSecurityContext(); err != nil {
		return err
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
		internalConfig = defaultConfig.DeepCopy()
	} else {
		syncConfigFrom(config)
	}

	defaultRoutingSuffix, err := discoverRouteSuffix(client)
	if err != nil {
		return err
	}
	defaultConfig.Routing.ClusterHostSuffix = defaultRoutingSuffix
	if internalConfig.Routing.ClusterHostSuffix == "" {
		internalConfig.Routing.ClusterHostSuffix = defaultRoutingSuffix
	}

	clusterProxy, err := proxy.GetClusterProxyConfig(client)
	if err != nil {
		return err
	}
	defaultConfig.Routing.ProxyConfig = clusterProxy
	internalConfig.Routing.ProxyConfig = proxy.MergeProxyConfigs(clusterProxy, internalConfig.Routing.ProxyConfig)

	logCurrentConfig()
	return nil
}

func IsSetUp() bool {
	return internalConfig != nil
}

func ExperimentalFeaturesEnabled() bool {
	if internalConfig == nil || internalConfig.EnableExperimentalFeatures == nil {
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

func getMergedConfig(from, to *controller.OperatorConfiguration) *controller.OperatorConfiguration {
	mergedConfig := to.DeepCopy()
	fromCopy := from.DeepCopy()
	mergeConfig(fromCopy, mergedConfig)
	return mergedConfig
}

func syncConfigFrom(newConfig *controller.DevWorkspaceOperatorConfig) {
	if newConfig == nil || newConfig.Name != OperatorConfigName || newConfig.Namespace != configNamespace {
		return
	}
	configMutex.Lock()
	defer configMutex.Unlock()
	internalConfig = defaultConfig.DeepCopy()
	mergeConfig(newConfig.Config, internalConfig)
	logCurrentConfig()
}

func restoreDefaultConfig() {
	configMutex.Lock()
	defer configMutex.Unlock()
	internalConfig = defaultConfig.DeepCopy()
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
		if from.Workspace.ServiceAccount != nil {
			if to.Workspace.ServiceAccount == nil {
				to.Workspace.ServiceAccount = &controller.ServiceAccountConfig{}
			}
			if from.Workspace.ServiceAccount.ServiceAccountName != "" {
				to.Workspace.ServiceAccount.ServiceAccountName = from.Workspace.ServiceAccount.ServiceAccountName
			}
			if from.Workspace.ServiceAccount.DisableCreation != nil {
				to.Workspace.ServiceAccount.DisableCreation = pointer.Bool(*from.Workspace.ServiceAccount.DisableCreation)
			}
			if from.Workspace.ServiceAccount.ServiceAccountTokens != nil {
				to.Workspace.ServiceAccount.ServiceAccountTokens = from.Workspace.ServiceAccount.ServiceAccountTokens
			}
		}
		if from.Workspace.ImagePullPolicy != "" {
			to.Workspace.ImagePullPolicy = from.Workspace.ImagePullPolicy
		}
		if from.Workspace.DeploymentStrategy != "" {
			to.Workspace.DeploymentStrategy = from.Workspace.DeploymentStrategy
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
			to.Workspace.PodSecurityContext = mergePodSecurityContext(to.Workspace.PodSecurityContext, from.Workspace.PodSecurityContext)
		}
		if from.Workspace.ContainerSecurityContext != nil {
			to.Workspace.ContainerSecurityContext = mergeContainerSecurityContext(to.Workspace.ContainerSecurityContext, from.Workspace.ContainerSecurityContext)
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
		if from.Workspace.PersistUserHome != nil {
			if to.Workspace.PersistUserHome == nil {
				to.Workspace.PersistUserHome = &controller.PersistentHomeConfig{}
			}
			if from.Workspace.PersistUserHome.Enabled != nil {
				to.Workspace.PersistUserHome.Enabled = from.Workspace.PersistUserHome.Enabled
			}
		}
		if from.Workspace.DefaultTemplate != nil {
			templateSpecContentCopy := from.Workspace.DefaultTemplate.DeepCopy()
			to.Workspace.DefaultTemplate = templateSpecContentCopy
		}
		if from.Workspace.SchedulerName != "" {
			to.Workspace.SchedulerName = from.Workspace.SchedulerName
		}
		if from.Workspace.ProjectCloneConfig != nil {
			if to.Workspace.ProjectCloneConfig == nil {
				to.Workspace.ProjectCloneConfig = &controller.ProjectCloneConfig{}
			}
			if from.Workspace.ProjectCloneConfig.Image != "" {
				to.Workspace.ProjectCloneConfig.Image = from.Workspace.ProjectCloneConfig.Image
			}
			if from.Workspace.ProjectCloneConfig.ImagePullPolicy != "" {
				to.Workspace.ProjectCloneConfig.ImagePullPolicy = from.Workspace.ProjectCloneConfig.ImagePullPolicy
			}
			if from.Workspace.ProjectCloneConfig.Resources != nil {
				if to.Workspace.ProjectCloneConfig.Resources == nil {
					to.Workspace.ProjectCloneConfig.Resources = &corev1.ResourceRequirements{}
				}
				to.Workspace.ProjectCloneConfig.Resources = mergeResources(from.Workspace.ProjectCloneConfig.Resources, to.Workspace.ProjectCloneConfig.Resources)
			}

			// Overwrite env instead of trying to merge, don't want to bother merging lists when
			// the default is empty
			if from.Workspace.ProjectCloneConfig.Env != nil {
				to.Workspace.ProjectCloneConfig.Env = from.Workspace.ProjectCloneConfig.Env
			}
		}
		if from.Workspace.DefaultContainerResources != nil {
			if to.Workspace.DefaultContainerResources == nil {
				to.Workspace.DefaultContainerResources = &corev1.ResourceRequirements{}
			}
			to.Workspace.DefaultContainerResources = mergeResources(from.Workspace.DefaultContainerResources, to.Workspace.DefaultContainerResources)
		}
	}
}

func mergePodSecurityContext(base, patch *corev1.PodSecurityContext) *corev1.PodSecurityContext {
	baseBytes, err := json.Marshal(base)
	if err != nil {
		log.Info("Failed to serialize base pod security context: %s", err)
		return base
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		log.Info("Failed to serialize configured pod security context: %s", err)
		return base
	}
	patchedBytes, err := strategicpatch.StrategicMergePatch(baseBytes, patchBytes, &corev1.PodSecurityContext{})
	if err != nil {
		log.Info("Failed to merge configured pod security context: %s", err)
		return base
	}
	patched := &corev1.PodSecurityContext{}
	if err := json.Unmarshal(patchedBytes, patched); err != nil {
		log.Info("Failed to deserialize patched pod security context: %s", patched)
		return base
	}
	return patched
}

func mergeContainerSecurityContext(base, patch *corev1.SecurityContext) *corev1.SecurityContext {
	baseBytes, err := json.Marshal(base)
	if err != nil {
		log.Info("Failed to serialize base container security context: %s", err)
		return base
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		log.Info("Failed to serialize configured container security context: %s", err)
		return base
	}
	patchedBytes, err := strategicpatch.StrategicMergePatch(baseBytes, patchBytes, &corev1.SecurityContext{})
	if err != nil {
		log.Info("Failed to merge configured container security context: %s", err)
		return base
	}
	patched := &corev1.SecurityContext{}
	if err := json.Unmarshal(patchedBytes, patched); err != nil {
		log.Info("Failed to deserialize patched container security context: %s", patched)
		return base
	}
	return patched
}

func mergeResources(from, to *corev1.ResourceRequirements) *corev1.ResourceRequirements {
	result := to.DeepCopy()
	if from.Limits != nil {
		if result.Limits == nil {
			result.Limits = corev1.ResourceList{}
		}
		if cpu, ok := from.Limits[corev1.ResourceCPU]; ok {
			result.Limits[corev1.ResourceCPU] = cpu
		}
		if memory, ok := from.Limits[corev1.ResourceMemory]; ok {
			result.Limits[corev1.ResourceMemory] = memory
		}
	}
	if from.Requests != nil {
		if result.Requests == nil {
			result.Requests = corev1.ResourceList{}
		}
		if cpu, ok := from.Requests[corev1.ResourceCPU]; ok {
			result.Requests[corev1.ResourceCPU] = cpu
		}
		if memory, ok := from.Requests[corev1.ResourceMemory]; ok {
			result.Requests[corev1.ResourceMemory] = memory
		}
	}
	return result
}

func GetCurrentConfigString(currConfig *controller.OperatorConfiguration) string {
	if currConfig == nil {
		return ""
	}

	routing := currConfig.Routing
	var config []string
	if routing != nil {
		if routing.ClusterHostSuffix != "" && routing.ClusterHostSuffix != defaultConfig.Routing.ClusterHostSuffix {
			config = append(config, fmt.Sprintf("routing.clusterHostSuffix=%s", routing.ClusterHostSuffix))
		}
		if routing.DefaultRoutingClass != defaultConfig.Routing.DefaultRoutingClass {
			config = append(config, fmt.Sprintf("routing.defaultRoutingClass=%s", routing.DefaultRoutingClass))
		}
	}
	workspace := currConfig.Workspace
	if workspace != nil {
		if workspace.ImagePullPolicy != defaultConfig.Workspace.ImagePullPolicy {
			config = append(config, fmt.Sprintf("workspace.imagePullPolicy=%s", workspace.ImagePullPolicy))
		}
		if workspace.DeploymentStrategy != defaultConfig.Workspace.DeploymentStrategy {
			config = append(config, fmt.Sprintf("workspace.deploymentStrategy=%s", workspace.DeploymentStrategy))
		}
		if workspace.PVCName != defaultConfig.Workspace.PVCName {
			config = append(config, fmt.Sprintf("workspace.pvcName=%s", workspace.PVCName))
		}
		if workspace.ServiceAccount != nil {
			if workspace.ServiceAccount.ServiceAccountName != defaultConfig.Workspace.ServiceAccount.ServiceAccountName {
				config = append(config, fmt.Sprintf("workspace.serviceAccount.serviceAccountName=%s", workspace.ServiceAccount.ServiceAccountName))
			}
			if workspace.ServiceAccount.DisableCreation != nil && *workspace.ServiceAccount.DisableCreation != *defaultConfig.Workspace.ServiceAccount.DisableCreation {
				config = append(config, fmt.Sprintf("workspace.serviceAccount.disableCreation=%t", *workspace.ServiceAccount.DisableCreation))
			}
			if workspace.ServiceAccount.ServiceAccountTokens != nil {
				serviceAccountTokens := make([]string, 0)
				for _, saToken := range workspace.ServiceAccount.ServiceAccountTokens {
					serviceAccountTokens = append(serviceAccountTokens, saToken.String())
				}
				config = append(config, fmt.Sprintf("workspace.serviceAccount.serviceAccountTokens=[%s]", strings.Join(serviceAccountTokens, ", ")))
			}
		}
		if workspace.StorageClassName != nil && workspace.StorageClassName != defaultConfig.Workspace.StorageClassName {
			config = append(config, fmt.Sprintf("workspace.storageClassName=%s", *workspace.StorageClassName))
		}
		if workspace.IdleTimeout != defaultConfig.Workspace.IdleTimeout {
			config = append(config, fmt.Sprintf("workspace.idleTimeout=%s", workspace.IdleTimeout))
		}
		if workspace.ProgressTimeout != "" && workspace.ProgressTimeout != defaultConfig.Workspace.ProgressTimeout {
			config = append(config, fmt.Sprintf("workspace.progressTimeout=%s", workspace.ProgressTimeout))
		}
		if workspace.IgnoredUnrecoverableEvents != nil {
			config = append(config, fmt.Sprintf("workspace.ignoredUnrecoverableEvents=%s",
				strings.Join(workspace.IgnoredUnrecoverableEvents, ";")))
		}
		if workspace.CleanupOnStop != nil && *workspace.CleanupOnStop != *defaultConfig.Workspace.CleanupOnStop {
			config = append(config, fmt.Sprintf("workspace.cleanupOnStop=%t", *workspace.CleanupOnStop))
		}
		if workspace.DefaultStorageSize != nil {
			if workspace.DefaultStorageSize.Common != nil && workspace.DefaultStorageSize.Common.String() != defaultConfig.Workspace.DefaultStorageSize.Common.String() {
				config = append(config, fmt.Sprintf("workspace.defaultStorageSize.common=%s", workspace.DefaultStorageSize.Common.String()))
			}
			if workspace.DefaultStorageSize.PerWorkspace != nil && workspace.DefaultStorageSize.PerWorkspace.String() != defaultConfig.Workspace.DefaultStorageSize.PerWorkspace.String() {
				config = append(config, fmt.Sprintf("workspace.defaultStorageSize.perWorkspace=%s", workspace.DefaultStorageSize.PerWorkspace.String()))
			}
		}
		if workspace.PersistUserHome != nil {
			if workspace.PersistUserHome.Enabled != nil && *workspace.PersistUserHome.Enabled != *defaultConfig.Workspace.PersistUserHome.Enabled {
				config = append(config, fmt.Sprintf("workspace.persistUserHome.enabled=%t", *workspace.PersistUserHome.Enabled))
			}
		}
		if !reflect.DeepEqual(workspace.PodSecurityContext, defaultConfig.Workspace.PodSecurityContext) {
			config = append(config, "workspace.podSecurityContext is set")
		}
		if !reflect.DeepEqual(workspace.ContainerSecurityContext, defaultConfig.Workspace.ContainerSecurityContext) {
			config = append(config, "workspace.containerSecurityContext is set")
		}
		if workspace.DefaultTemplate != nil {
			config = append(config, "workspace.defaultTemplate is set")
		}
		if workspace.SchedulerName != "" {
			config = append(config, fmt.Sprintf("workspace.schedulerName=%s", workspace.SchedulerName))
		}
		if workspace.ProjectCloneConfig != nil {
			if workspace.ProjectCloneConfig.Image != defaultConfig.Workspace.ProjectCloneConfig.Image {
				config = append(config, fmt.Sprintf("workspace.projectClone.image=%s", workspace.ProjectCloneConfig.Image))
			}
			if workspace.ProjectCloneConfig.ImagePullPolicy != defaultConfig.Workspace.ProjectCloneConfig.ImagePullPolicy {
				config = append(config, fmt.Sprintf("workspace.projectClone.imagePullPolicy=%s", workspace.ProjectCloneConfig.ImagePullPolicy))
			}
			if workspace.ProjectCloneConfig.Env != nil {
				config = append(config, "workspace.projectClone.env is set")
			}
			if !reflect.DeepEqual(workspace.ProjectCloneConfig.Resources, defaultConfig.Workspace.ProjectCloneConfig.Resources) {
				config = append(config, "workspace.projectClone.resources is set")
			}
		}
		if !reflect.DeepEqual(workspace.DefaultContainerResources, defaultConfig.Workspace.DefaultContainerResources) {
			config = append(config, "workspace.defaultContainerResources is set")
		}
	}
	if currConfig.EnableExperimentalFeatures != nil && *currConfig.EnableExperimentalFeatures {
		config = append(config, "enableExperimentalFeatures=true")
	}
	if len(config) == 0 {
		return ""
	} else {
		return strings.Join(config, ",")
	}
}

// logCurrentConfig formats the current operator configuration as a plain string
func logCurrentConfig() {
	currConfig := GetCurrentConfigString(internalConfig)
	if len(currConfig) == 0 {
		log.Info("Updated config to [(default config)]")
	} else {
		log.Info(fmt.Sprintf("Updated config to [%s]", currConfig))
	}

	if internalConfig.Routing.ProxyConfig != nil {
		log.Info("Resolved proxy configuration", "proxy", internalConfig.Routing.ProxyConfig)
	}
}
