//
// Copyright (c) 2019-2020 Red Hat, Inc.
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
	"errors"
	"github.com/che-incubator/che-workspace-operator/internal/cluster"
	"os"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"strings"

	routeV1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"fmt"
)

var ControllerCfg ControllerConfig
var log = logf.Log.WithName("controller_workspace_config")

const (
	ConfigMapNameEnvVar      = "CONTROLLER_CONFIG_MAP_NAME"
	ConfigMapNamespaceEnvVar = "CONTROLLER_CONFIG_MAP_NAMESPACE"
)

var ConfigMapReference = client.ObjectKey{
	Namespace: "",
	Name:      "che-workspace-controller",
}

type ControllerConfig struct {
	configMap   *corev1.ConfigMap
	isOpenShift bool
}

func (wc *ControllerConfig) update(configMap *corev1.ConfigMap) {
	log.Info("Updating the configuration from config map '%s' in namespace '%s'", configMap.Name, configMap.Namespace)
	wc.configMap = configMap
}

func (wc *ControllerConfig) GetWorkspacePVCName() string {
	return wc.GetPropertyOrDefault(workspacePVCName, defaultWorkspacePVCName)
}

func (wc *ControllerConfig) GetDefaultRoutingClass() string {
	return wc.GetPropertyOrDefault(routingClass, defaultRoutingClass)
}

func (wc *ControllerConfig) GetPluginRegistry() string {
	return wc.GetPropertyOrDefault(pluginRegistryURL, "")
}

func (wc *ControllerConfig) GetIngressGlobalDomain() string {
	return wc.GetPropertyOrDefault(ingressGlobalDomain, defaultIngressGlobalDomain)
}

func (wc *ControllerConfig) GetPVCStorageClassName() *string {
	return wc.GetProperty(workspacePVCStorageClassName)
}

func (wc *ControllerConfig) GetCheRestApisDockerImage() string {
	return wc.GetPropertyOrDefault(serverImageName, defaultServerImageName)
}

func (wc *ControllerConfig) IsOpenShift() bool {
	return wc.isOpenShift
}

func (wc *ControllerConfig) SetIsOpenShift(isOpenShift bool) {
	wc.isOpenShift = isOpenShift
}

func (wc *ControllerConfig) GetSidecarPullPolicy() string {
	return wc.GetPropertyOrDefault(sidecarPullPolicy, defaultSidecarPullPolicy)
}

func (wc *ControllerConfig) GetPluginArtifactsBrokerImage() string {
	return wc.GetPropertyOrDefault(pluginArtifactsBrokerImage, defaultPluginArtifactsBrokerImage)
}

func (wc *ControllerConfig) GetWebhooksEnabled() string {
	return wc.GetPropertyOrDefault(webhooksEnabled, defaultWebhooksEnabled)
}

func (wc *ControllerConfig) GetProperty(name string) *string {
	val, exists := wc.configMap.Data[name]
	if exists {
		return &val
	}
	return nil
}

func (wc *ControllerConfig) GetPropertyOrDefault(name string, defaultValue string) string {
	val, exists := wc.configMap.Data[name]
	if exists {
		return val
	}
	return defaultValue
}

func updateConfigMap(client client.Client, meta metav1.Object, obj runtime.Object) {
	if meta.GetNamespace() != ConfigMapReference.Namespace ||
		meta.GetName() != ConfigMapReference.Name {
		return
	}
	if cm, isConfigMap := obj.(*corev1.ConfigMap); isConfigMap {
		ControllerCfg.update(cm)
		return
	}

	configMap := &corev1.ConfigMap{}
	err := client.Get(context.TODO(), ConfigMapReference, configMap)
	if err != nil {
		log.Error(err, fmt.Sprintf("Cannot find the '%s' ConfigMap in namespace '%s'", ConfigMapReference.Name, ConfigMapReference.Namespace))
	}
	ControllerCfg.update(configMap)
}

func WatchControllerConfig(ctr controller.Controller, mgr manager.Manager) error {
	customConfig := false
	configMapName, found := os.LookupEnv(ConfigMapNameEnvVar)
	if found && len(configMapName) > 0 {
		ConfigMapReference.Name = configMapName
		customConfig = true
	}
	configMapNamespace, found := os.LookupEnv(ConfigMapNamespaceEnvVar)
	if found && len(configMapNamespace) > 0 {
		ConfigMapReference.Namespace = configMapNamespace
		customConfig = true
	}

	if ConfigMapReference.Namespace == "" {
		return errors.New(fmt.Sprintf("You should set the namespace of the controller config map through the '%s' environment variable", ConfigMapNamespaceEnvVar))
	}

	configMap := &corev1.ConfigMap{}
	nonCachedClient, err := client.New(mgr.GetConfig(), client.Options{})
	if err != nil {
		return err
	}
	log.Info(fmt.Sprintf("Searching for config map '%s' in namespace '%s'", ConfigMapReference.Name, ConfigMapReference.Namespace))
	err = nonCachedClient.Get(context.TODO(), ConfigMapReference, configMap)
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			return err
		}
		if customConfig {
			return errors.New(fmt.Sprintf("Cannot find the '%s' ConfigMap in namespace '%s'", ConfigMapReference.Name, ConfigMapReference.Namespace))
		}

		buildDefaultConfigMap(configMap)

		err = nonCachedClient.Create(context.TODO(), configMap)
		if err != nil {
			return err
		}
		log.Info(fmt.Sprintf("  => created config map '%s' in namespace '%s'", configMap.GetObjectMeta().GetName(), configMap.GetObjectMeta().GetNamespace()))
	} else {
		log.Info(fmt.Sprintf("  => found config map '%s' in namespace '%s'", configMap.GetObjectMeta().GetName(), configMap.GetObjectMeta().GetNamespace()))
	}

	err = fillOpenShiftRouteSuffixIfNecessary(nonCachedClient, configMap)
	if err != nil {
		return err
	}

	updateConfigMap(nonCachedClient, configMap.GetObjectMeta(), configMap)

	var emptyMapper handler.ToRequestsFunc = func(obj handler.MapObject) []reconcile.Request {
		return []reconcile.Request{}
	}
	err = ctr.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: emptyMapper,
	}, predicate.Funcs{
		UpdateFunc: func(evt event.UpdateEvent) bool {
			updateConfigMap(mgr.GetClient(), evt.MetaNew, evt.ObjectNew)
			return false
		},
		CreateFunc: func(evt event.CreateEvent) bool {
			updateConfigMap(mgr.GetClient(), evt.Meta, evt.Object)
			return false
		},
		DeleteFunc: func(evt event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(evt event.GenericEvent) bool {
			return false
		},
	})

	return err
}

func buildDefaultConfigMap(cm *corev1.ConfigMap) {
	cm.Name = ConfigMapReference.Name
	cm.Namespace = ConfigMapReference.Namespace

	cm.Data = map[string]string{
		ingressGlobalDomain:        defaultIngressGlobalDomain,
		pluginArtifactsBrokerImage: defaultPluginArtifactsBrokerImage,
	}
}

func fillOpenShiftRouteSuffixIfNecessary(nonCachedClient client.Client, configMap *corev1.ConfigMap) error {
	isOS, err := cluster.IsOpenShift()
	if err != nil {
		return err
	}
	if !isOS {
		return nil
	}
	testRoute := &routeV1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: configMap.Namespace,
			Name:      "che-workspace-controller-test-route",
		},
		Spec: routeV1.RouteSpec{
			To: routeV1.RouteTargetReference{
				Kind: "Service",
				Name: "che-workspace-controller-test-route",
			},
		},
	}

	err = nonCachedClient.Create(context.TODO(), testRoute)
	if err != nil {
		return err
	}
	defer nonCachedClient.Delete(context.TODO(), testRoute)
	host := testRoute.Spec.Host
	if host != "" {
		prefixToRemove := "che-workspace-controller-test-route-" + configMap.Namespace + "."
		configMap.Data["ingress.global.domain"] = strings.TrimPrefix(host, prefixToRemove)
	}

	err = nonCachedClient.Update(context.TODO(), configMap)
	if err != nil {
		return err
	}

	return nil
}
