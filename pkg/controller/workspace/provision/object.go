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

package provision

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SyncMutableObjects synchronizes runtime objects and changes/updates existing ones
func SyncMutableObjects(objects []runtime.Object, client client.Client, reqLogger logr.Logger) error {
	for _, object := range objects {
		if err := SyncMutableObject(object, client, reqLogger); err != nil {
			return err
		}
	}
	return nil
}

// SyncMutableObject synchronizes a runtime object and changes/updates existing ones
func SyncMutableObject(object runtime.Object, client client.Client, reqLogger logr.Logger) error {
	prereqAsMetaObject, isMeta := object.(metav1.Object)
	if !isMeta {
		return errors.NewBadRequest("Converted objects are not valid K8s objects")
	}
	err := SyncObject(object, client, reqLogger)

	// If the object already exists we can update it
	if !errors.IsNotFound(err) {
		reqLogger.Info("    => Updating "+reflect.TypeOf(prereqAsMetaObject).Elem().String(), "namespace", prereqAsMetaObject.GetNamespace(), "name", prereqAsMetaObject.GetName())
		err = client.Update(context.TODO(), object)
	}
	return nil
}

// SyncObject synchronizes runtime objects but does not change/updating existing ones
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
	return nil
}
