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
	"os"
	"strings"

	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"

	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	routeV1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
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

func (wc *ControllerConfig) GetWorkspacePVCName() string {
	return wc.GetPropertyOrDefault(workspacePVCName, defaultWorkspacePVCName)
}

func (wc *ControllerConfig) GetDefaultRoutingClass() string {
	return wc.GetPropertyOrDefault(routingClass, defaultRoutingClass)
}

//GetExperimentalFeaturesEnabled returns true if experimental features should be enabled.
//DO NOT TURN ON IT IN THE PRODUCTION.
//Experimental features are not well tested and may be totally removed without announcement.
func (wc *ControllerConfig) GetExperimentalFeaturesEnabled() bool {
	return wc.GetPropertyOrDefault(experimentalFeaturesEnabled, defaultExperimentalFeaturesEnabled) == "true"
}

func (wc *ControllerConfig) GetPVCStorageClassName() *string {
	return wc.GetProperty(workspacePVCStorageClassName)
}

func (wc *ControllerConfig) GetSidecarPullPolicy() string {
	return wc.GetPropertyOrDefault(sidecarPullPolicy, defaultSidecarPullPolicy)
}

func (wc *ControllerConfig) GetTlsInsecureSkipVerify() string {
	return wc.GetPropertyOrDefault(tlsInsecureSkipVerify, defaultTlsInsecureSkipVerify)
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

func (wc *ControllerConfig) Validate() error {
	return nil
}

func (wc *ControllerConfig) GetWorkspaceIdleTimeout() string {
	return wc.GetPropertyOrDefault(devworkspaceIdleTimeout, defaultDevWorkspaceIdleTimeout)
}

func (wc *ControllerConfig) GetWorkspaceControllerSA() (string, error) {
	saName := os.Getenv(constants.ControllerServiceAccountNameEnvVar)
	if saName == "" {
		return "", fmt.Errorf("could not get service account name")
	}
	return saName, nil
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

func WatchControllerConfig(mgr manager.Manager) error {
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
		return fmt.Errorf("you should set the namespace of the controller config map through the '%s' environment variable", ConfigMapNamespaceEnvVar)
	}

	configMap := &corev1.ConfigMap{}
	nonCachedClient, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: mgr.GetScheme(),
	})
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
			return fmt.Errorf("cannot find the '%s' ConfigMap in namespace '%s'", ConfigMapReference.Name, ConfigMapReference.Namespace)
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

	if configMap.Data == nil {
		configMap.Data = map[string]string{}
	}
	err = fillOpenShiftRouteSuffixIfNecessary(nonCachedClient, configMap)
	if err != nil {
		return err
	}

	updateConfigMap(nonCachedClient, configMap.GetObjectMeta(), configMap)

	return nil
}

func SetupConfigForTesting(cm *corev1.ConfigMap) {
	ControllerCfg.update(cm)
}

func buildDefaultConfigMap(cm *corev1.ConfigMap) {
	cm.Name = ConfigMapReference.Name
	cm.Namespace = ConfigMapReference.Namespace
	cm.Labels = constants.ControllerAppLabels()

	cm.Data = map[string]string{}
}

func fillOpenShiftRouteSuffixIfNecessary(nonCachedClient client.Client, configMap *corev1.ConfigMap) error {
	if !infrastructure.IsOpenShift() {
		return nil
	}

	testRoute := &routeV1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: configMap.Namespace,
			Name:      "devworkspace-controller-test-route",
		},
		Spec: routeV1.RouteSpec{
			To: routeV1.RouteTargetReference{
				Kind: "Service",
				Name: "devworkspace-controller-test-route",
			},
		},
	}

	err := nonCachedClient.Create(context.TODO(), testRoute)
	if err != nil {
		return err
	}
	defer nonCachedClient.Delete(context.TODO(), testRoute)
	host := testRoute.Spec.Host
	if host != "" {
		prefixToRemove := "devworkspace-controller-test-route-" + configMap.Namespace + "."
		configMap.Data[RoutingSuffix] = strings.TrimPrefix(host, prefixToRemove)
	}

	err = nonCachedClient.Update(context.TODO(), configMap)
	if err != nil {
		return err
	}

	return nil
}

func ConfigMapPredicates(mgr manager.Manager) predicate.Predicate {
	return predicate.Funcs{
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
	}
}
