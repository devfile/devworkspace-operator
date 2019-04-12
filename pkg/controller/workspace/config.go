// MY LICENSEdep

package workspace

import (
	"context"
	"errors"
	"os"

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
)

var workspaceConfig WorkspaceConfig

var (
	ConfigMapNameEnvVar      = "CONTROLLER_CONGIG_MAP_NAME"
	ConfigMapNamespaceEnvVar = "CONTROLLER_CONGIG_MAP_NAMESPACE"
)

type WorkspaceConfig struct {
	configMap *corev1.ConfigMap
}

func (wc *WorkspaceConfig) update(configMap *corev1.ConfigMap) {
	log.Info(join("", "Updating the configuration from config map '", configMap.Name, "' in namespace '", configMap.Namespace, "'"))
	wc.configMap = configMap
}

func (wc *WorkspaceConfig) getPluginRegistry() string {
	return *wc.getProperty("plugin.registry")
}

func (wc *WorkspaceConfig) getIngressGlobalDomain() string {
	return *wc.getProperty("ingress.global.domain")
}

func (wc *WorkspaceConfig) getPVCStorageClassName() *string {
	return wc.getProperty("pvc.storageclass.name")
}

func (wc *WorkspaceConfig) getCheRestApisDockerImage() string {
	optional := wc.getProperty("pvc.storageclass.name")
	if optional == nil {
		return "quay.io/che-incubator/che-workspace-crd-rest-apis:latest"
	}
	return *optional
}

func (wc *WorkspaceConfig) getProperty(name string) *string {
	val, exists := wc.configMap.Data[name]
	if exists {
		return &val
	}
	return nil
}

func updateConfigMap(client client.Client, meta metav1.Object, obj runtime.Object) {
	if meta.GetNamespace() != configMapReference.Namespace ||
		meta.GetName() != configMapReference.Name {
		log.Info(join("", "Available config map '", meta.GetNamespace(), "/", meta.GetName(), "' doesn't match the expected '", configMapReference.Name, "' ConfigMap in namespace '", configMapReference.Namespace, "'"))
		return
	}
	if cm, isConfigMap := obj.(*corev1.ConfigMap); isConfigMap {
		workspaceConfig.update(cm)
		return
	}

	configMap := &corev1.ConfigMap{}
	err := client.Get(context.TODO(), configMapReference, configMap)
	if err != nil {
		log.Error(err, join("", "Cannot find the '", configMapReference.Name, "' ConfigMap in namespace '", configMapReference.Namespace, "'"))
	}
	workspaceConfig.update(configMap)
}

func watchWorkspaceConfig(ctr controller.Controller, mgr manager.Manager) error {
	configMapName, found := os.LookupEnv(ConfigMapNameEnvVar)
	if found && len(configMapName) > 0 {
		configMapReference.Name = configMapName
	}
	configMapNamespace, found := os.LookupEnv(ConfigMapNamespaceEnvVar)
	if found && len(configMapNamespace) > 0 {
		configMapReference.Namespace = configMapNamespace
	}

	configMap := &corev1.ConfigMap{}
	nonCachedClient, err := client.New(mgr.GetConfig(), client.Options{})
	if err != nil {
		return err
	}
	log.Info(join("", "Searching for config map '", configMapReference.Name, "' in namespace '", configMapReference.Namespace, "'"))
	err = nonCachedClient.Get(context.TODO(), configMapReference, configMap)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return errors.New(join("", "Cannot find the '", configMapReference.Name, "' ConfigMap in namespace '", configMapReference.Namespace, "'"))
		}
		return err
	}

	log.Info(join("", "  => found config map '", configMap.GetObjectMeta().GetName(), "' in namespace '", configMap.GetObjectMeta().GetNamespace(), "'"))

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
