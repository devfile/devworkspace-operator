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
	"fmt"
	"reflect"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Map to store diff options for each type we're handling.
var diffOpts = map[reflect.Type]cmp.Options{
	reflect.TypeOf(rbacv1.Role{}): {cmpopts.IgnoreFields(rbacv1.Role{}, "TypeMeta", "ObjectMeta")},
	reflect.TypeOf(rbacv1.RoleBinding{}): {
		cmpopts.IgnoreFields(rbacv1.RoleBinding{}, "TypeMeta", "ObjectMeta"),
		cmpopts.IgnoreFields(rbacv1.RoleRef{}, "APIGroup"),
		cmpopts.IgnoreFields(rbacv1.Subject{}, "APIGroup"),
	},
}

// SyncMutableObjects synchronizes runtime objects and changes/updates existing ones
func SyncMutableObjects(objects []runtime.Object, client client.Client, reqLogger logr.Logger) (didChange bool, err error) {
	didAnyChange := false
	for _, object := range objects {
		didChange, err := SyncObject(object, client, reqLogger, true)
		if err != nil {
			return false, err
		}
		didAnyChange = didAnyChange || didChange
	}
	return didAnyChange, nil
}

// SyncObject synchronizes a runtime object and changes/updates existing ones
func SyncObject(object runtime.Object, client client.Client, reqLogger logr.Logger, update bool) (didChange bool, apiErr error) {
	objMeta, isMeta := object.(metav1.Object)
	if !isMeta {
		return false, errors.NewBadRequest("Converted objects are not valid K8s objects")
	}

	objType := reflect.TypeOf(object).Elem()

	reqLogger.V(1).Info("Managing K8s Object", "kind", objType.String(), "name", objMeta.GetName())

	found := reflect.New(objType).Interface().(runtime.Object)
	err := client.Get(context.TODO(), types.NamespacedName{Name: objMeta.GetName(), Namespace: objMeta.GetNamespace()}, found)
	if err != nil {
		if !errors.IsNotFound(err) {
			return false, err
		}
		reqLogger.Info("    => Creating "+objType.String(), "namespace", objMeta.GetNamespace(), "name", objMeta.GetName())
		createErr := client.Create(context.TODO(), object)
		if errors.IsAlreadyExists(createErr) {
			fmt.Println("Suppressing alreadyExists in ", objType.String())
			return true, nil
		}
		return true, createErr
	}
	if !update {
		return false, nil
	}

	diffOpt, ok := diffOpts[objType]
	if !ok {
		reqLogger.V(0).Info("WARN: Could not get diff options for element " + objType.String())
		diffOpt = cmp.Options{}
	}
	if !cmp.Equal(object, found, diffOpt) {
		reqLogger.Info("    => Updating "+objType.String(), "namespace", objMeta.GetNamespace(), "name", objMeta.GetName())
		updateErr := client.Update(context.TODO(), object)
		if errors.IsConflict(updateErr) {
			fmt.Println("Suppressing conflict in ", objType.String())
			return true, nil
		}
		return true, updateErr
	}
	return false, nil
}
