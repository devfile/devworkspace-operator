package provision

import (
	"context"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SyncObject synchronize the runtime objects
func SyncObjects(objects []runtime.Object, client client.Client, reqLogger logr.Logger) error {
	for _, object := range objects {
		if err := SyncObject(object, client, reqLogger); err != nil {
			return err
		}
	}
	return nil
}

// SyncObject synchronize the runtime object
func SyncObject(object runtime.Object, client client.Client, reqLogger logr.Logger) error {
	prereqAsMetaObject, isMeta := object.(metav1.Object)
	if !isMeta {
		return errors.NewBadRequest("Converted objects are not valid K8s objects")
	}

	reqLogger.V(1).Info("Managing K8s Object", "kind", reflect.TypeOf(object).Elem().String(), "name", prereqAsMetaObject.GetName())

	found := reflect.New(reflect.TypeOf(object).Elem()).Interface().(runtime.Object)
	err := client.Get(context.TODO(), types.NamespacedName{Name: prereqAsMetaObject.GetName(), Namespace: prereqAsMetaObject.GetNamespace()}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("    => Creating "+reflect.TypeOf(prereqAsMetaObject).Elem().String(), "namespace", prereqAsMetaObject.GetNamespace(), "name", prereqAsMetaObject.GetName())
		err = client.Create(context.TODO(), object)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	//TODO Object is fetched but the state is not checked. Try to update it!
	return nil
}
