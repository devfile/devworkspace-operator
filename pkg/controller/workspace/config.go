// MY LICENSEdep

package workspace

import (
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
)

var controllerConfig ControllerConfig

var (
	ConfigMapNameEnvVar      = "CONTROLLER_CONGIG_MAP_NAME"
	ConfigMapNamespaceEnvVar = "CONTROLLER_CONGIG_MAP_NAMESPACE"
)

type ControllerConfig struct {
	configMap             *corev1.ConfigMap
	controllerIsOpenshift bool
}

func (wc *ControllerConfig) update(configMap *corev1.ConfigMap) {
	log.Info(join("", "Updating the configuration from config map '", configMap.Name, "' in namespace '", configMap.Namespace, "'"))
	wc.configMap = configMap
}

func (wc *ControllerConfig) getPluginRegistry() string {
	return *wc.getProperty("plugin.registry")
}

func (wc *ControllerConfig) getIngressGlobalDomain() string {
	return *wc.getProperty("ingress.global.domain")
}

func (wc *ControllerConfig) getPVCStorageClassName() *string {
	return wc.getProperty("pvc.storageclass.name")
}

func (wc *ControllerConfig) getCheRestApisDockerImage() string {
	optional := wc.getProperty("cherestapis.image.name")
	if optional == nil {
		return "quay.io/che-incubator/che-workspace-crd-rest-apis:latest"
	}
	return *optional
}

func (wc *ControllerConfig) isOpenshift() bool {
	return wc.controllerIsOpenshift
}

func (wc *ControllerConfig) getProperty(name string) *string {
	val, exists := wc.configMap.Data[name]
	if exists {
		return &val
	}
	return nil
}

func updateConfigMap(client client.Client, meta metav1.Object, obj runtime.Object) {
	if meta.GetNamespace() != configMapReference.Namespace ||
		meta.GetName() != configMapReference.Name {
		return
	}
	if cm, isConfigMap := obj.(*corev1.ConfigMap); isConfigMap {
		controllerConfig.update(cm)
		return
	}

	configMap := &corev1.ConfigMap{}
	err := client.Get(context.TODO(), configMapReference, configMap)
	if err != nil {
		log.Error(err, join("", "Cannot find the '", configMapReference.Name, "' ConfigMap in namespace '", configMapReference.Namespace, "'"))
	}
	controllerConfig.update(configMap)
}

func watchControllerConfig(ctr controller.Controller, mgr manager.Manager) error {
	customConfig := false
	configMapName, found := os.LookupEnv(ConfigMapNameEnvVar)
	if found && len(configMapName) > 0 {
		configMapReference.Name = configMapName
		customConfig = true
	}
	configMapNamespace, found := os.LookupEnv(ConfigMapNamespaceEnvVar)
	if found && len(configMapNamespace) > 0 {
		configMapReference.Namespace = configMapNamespace
		customConfig = true
	}

	if configMapReference.Namespace == "" {
		return errors.New(join("", "You should set the namespace of the controller config map through the '", ConfigMapNamespaceEnvVar, "' environment variable"))
	}

	configMap := &corev1.ConfigMap{}
	nonCachedClient, err := client.New(mgr.GetConfig(), client.Options{})
	if err != nil {
		return err
	}
	log.Info(join("", "Searching for config map '", configMapReference.Name, "' in namespace '", configMapReference.Namespace, "'"))
	err = nonCachedClient.Get(context.TODO(), configMapReference, configMap)
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			return err
		}
		if customConfig {
			return errors.New(join("", "Cannot find the '", configMapReference.Name, "' ConfigMap in namespace '", configMapReference.Namespace, "'"))
		}

		buildDefaultConfigMap(configMap)

		err = nonCachedClient.Create(context.TODO(), configMap)
		if err != nil {
			return err
		}
		log.Info(join("", "  => created config map '", configMap.GetObjectMeta().GetName(), "' in namespace '", configMap.GetObjectMeta().GetNamespace(), "'"))
	} else {
		log.Info(join("", "  => found config map '", configMap.GetObjectMeta().GetName(), "' in namespace '", configMap.GetObjectMeta().GetNamespace(), "'"))
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
	cm.Name = configMapReference.Name
	cm.Namespace = configMapReference.Namespace
	cm.Data = map[string]string{
		"ingress.global.domain":                    "",
		"plugin.registry":                          "https://che-plugin-registry.openshift.io",
		"che.workspace.plugin_broker.image":        "eclipse/che-plugin-broker:v0.15.0",
		"che.workspace.plugin_broker.theia.image":  "eclipse/che-theia-plugin-broker:v0.15.0",
		"che.workspace.plugin_broker.init.image":   "eclipse/che-init-plugin-broker:v0.15.0",
		"che.workspace.plugin_broker.vscode.image": "eclipse/che-vscode-extension-broker:v0.15.0",
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
