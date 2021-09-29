//
// Copyright (c) 2019-2021 Red Hat, Inc.
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
package workspace

import (
	"context"
	"fmt"
	"reflect"

	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
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
func SyncMutableObjects(objects []runtimeClient.Object, client runtimeClient.Client, reqLogger logr.Logger) (requeue bool, err error) {
	for _, object := range objects {
		_, shouldRequeue, err := SyncObject(object, client, reqLogger, true)
		if err != nil {
			return false, err
		}
		requeue = requeue || shouldRequeue
	}
	return requeue, nil
}

// SyncObject synchronizes a runtime object and changes/updates existing ones
func SyncObject(object runtimeClient.Object, client runtimeClient.Client, reqLogger logr.Logger, update bool) (clusterObject runtime.Object, requeue bool, apiErr error) {
	objMeta, isMeta := object.(metav1.Object)
	if !isMeta {
		return nil, true, errors.NewBadRequest("Converted objects are not valid K8s objects")
	}

	objType := reflect.TypeOf(object).Elem()

	reqLogger.V(1).Info("Managing K8s Object", "kind", objType.String(), "name", objMeta.GetName())

	found := reflect.New(objType).Interface().(runtimeClient.Object)
	err := client.Get(context.TODO(), types.NamespacedName{Name: objMeta.GetName(), Namespace: objMeta.GetNamespace()}, found)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, true, err
		}
		reqLogger.Info("Creating "+objType.String(), "namespace", objMeta.GetNamespace(), "name", objMeta.GetName())
		createErr := client.Create(context.TODO(), object)
		if errors.IsAlreadyExists(createErr) {
			return nil, true, nil
		}
		return nil, true, createErr
	}
	if !update {
		return found, false, nil
	}

	diffOpt, ok := diffOpts[objType]
	if !ok {
		reqLogger.V(0).Info("WARN: Could not get diff options for element " + objType.String())
		diffOpt = cmp.Options{}
	}
	if !cmp.Equal(object, found, diffOpt) {
		reqLogger.Info("Updating "+objType.String(), "namespace", objMeta.GetNamespace(), "name", objMeta.GetName())
		if config.ExperimentalFeaturesEnabled() {
			reqLogger.Info(fmt.Sprintf("Diff: %s", cmp.Diff(object, found, diffOpt)))
		}
		updateErr := client.Update(context.TODO(), object)
		if errors.IsConflict(updateErr) {
			return found, true, nil
		}
		return nil, true, updateErr
	}
	return found, false, nil
}
