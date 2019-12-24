// MY LICENSEdep

package config

import (
	"github.com/che-incubator/che-workspace-crd-operator/pkg/controller/registry"
	"strings"
	"context"
	"errors"
	"os"

	corev1 "k8s.io/api/core/v1"
	routeV1 "github.com/openshift/api/route/v1"
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

	. "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/utils"
	. "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/log"
	. "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/model"

	"fmt"
)

var ControllerCfg ControllerConfig

var (
	ConfigMapNameEnvVar      = "CONTROLLER_CONGIG_MAP_NAME"
	ConfigMapNamespaceEnvVar = "CONTROLLER_CONGIG_MAP_NAMESPACE"
)

var ConfigMapReference = client.ObjectKey{
	Namespace: "",
	Name:      "che-workspace-crd-controller",
}

type ControllerConfig struct {
	configMap             *corev1.ConfigMap
	ControllerIsOpenshift bool
}

func (wc *ControllerConfig) update(configMap *corev1.ConfigMap) {
	Log.Info("Updating the configuration from config map '%s' in namespace '%s'", configMap.Name, configMap.Namespace)
	wc.configMap = configMap
}

func (wc *ControllerConfig) GetPluginRegistry() string {
	optional := wc.GetProperty("plugin.registry")
	if optional != nil {
		return *optional
	}
	return registry.EmbeddedPluginRegistryUrl
}

func (wc *ControllerConfig) GetIngressGlobalDomain() string {
	return *wc.GetProperty("ingress.global.domain")
}

func (wc *ControllerConfig) GetPVCStorageClassName() *string {
	return wc.GetProperty("pvc.storageclass.name")
}

func (wc *ControllerConfig) GetCheRestApisDockerImage() string {
	optional := wc.GetProperty("cherestapis.image.name")
	if optional == nil {
		return "quay.io/che-incubator/che-workspace-crd-rest-apis:" + CheVersion
	}
	return *optional
}

func (wc *ControllerConfig) IsOpenshift() bool {
	return wc.ControllerIsOpenshift
}

func (wc *ControllerConfig) GetSidecarPullPolicy() string {
	optional := wc.GetProperty("sidecar.pull.policy")
	if optional == nil {
		return "IfNotPresent"
	}
	return *optional
}

func (wc *ControllerConfig) GetProperty(name string) *string {
	val, exists := wc.configMap.Data[name]
	if exists {
		return &val
	}
	return nil
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
		Log.Error(err, fmt.Sprintf("Cannot find the '%s' ConfigMap in namespace '%s'", ConfigMapReference.Name, ConfigMapReference.Namespace))
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
	Log.Info(fmt.Sprintf("Searching for config map '%s' in namespace '%s'", ConfigMapReference.Name, ConfigMapReference.Namespace))
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
		Log.Info(fmt.Sprintf("  => created config map '%s' in namespace '%s'", configMap.GetObjectMeta().GetName(), configMap.GetObjectMeta().GetNamespace()))
	} else {
		Log.Info(fmt.Sprintf("  => found config map '%s' in namespace '%s'", configMap.GetObjectMeta().GetName(), configMap.GetObjectMeta().GetNamespace()))
	}

	err = fillOpenshiftRouteSuffixIfNecessary(nonCachedClient, configMap)
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
		"ingress.global.domain":                      "",
		"che.workspace.plugin_broker.unified.image":  "eclipse/che-unified-plugin-broker:v0.20",
		"che.workspace.plugin_broker.init.image":     "eclipse/che-init-plugin-broker:v0.20",
	}
}

func fillOpenshiftRouteSuffixIfNecessary(nonCachedClient client.Client, configMap *corev1.ConfigMap) error {
	isOS, err := IsOpenShift()
	if err != nil {
		return err
	}
	if ! isOS {
		return nil
	}
	testRoute := &routeV1.Route {
		ObjectMeta: metav1.ObjectMeta {
			Namespace: configMap.Namespace,
			Name: "che-workspace-crd-test-route",
		},
		Spec: routeV1.RouteSpec {
				To: routeV1.RouteTargetReference {
					Kind: "Service",
					Name: "che-workspace-crd-test-route",
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
		prefixToRemove := "che-workspace-crd-test-route-" + configMap.Namespace + "."
		configMap.Data["ingress.global.domain"] = strings.TrimPrefix(host, prefixToRemove)
	}

	err = nonCachedClient.Update(context.TODO(), configMap)
	if err != nil {
		return err
	}

	return nil
}
