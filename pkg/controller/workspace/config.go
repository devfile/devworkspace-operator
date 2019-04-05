// MY LICENSEdep

package workspace

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var workspaceConfig WorkspaceConfig

type WorkspaceConfig struct {
	configMap *corev1.ConfigMap
}

func (wc *WorkspaceConfig) update(configMap *corev1.ConfigMap) {
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

func watchWorkspaceConfig(ctr controller.Controller, clt client.Client) error {
	var emptyMapper handler.ToRequestsFunc = func(obj handler.MapObject) []reconcile.Request {
		return []reconcile.Request{}
	}
	err := ctr.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: emptyMapper,
	}, predicate.Funcs{
		UpdateFunc: func(evt event.UpdateEvent) bool {
			updateConfigMap(clt, evt.MetaNew, evt.ObjectNew)
			return false
		},
		CreateFunc: func(evt event.CreateEvent) bool {
			updateConfigMap(clt, evt.Meta, evt.Object)
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
