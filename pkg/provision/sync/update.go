// Copyright (c) 2019-2022 Red Hat, Inc.
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

package sync

import (
	"errors"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// updateFunc returns the object that should be applied to the client.Update function
// when updating an object on the cluster. Typically, this will just be defaultUpdateFunc,
// which returns the spec obejct unmodified. However, some objects, such as Services, require
// fields to be copied over from the cluster object, e.g. .spec.clusterIP. If an updated object
// cannot be resolved, an error should be returned to signal that the object in question should
// be deleted instead.
//
// The 'cluster' argument may be specified as nil in the case where a cluster version of the
// spec object is inaccessible (not cached) and has to be handled specifically.
type updateFunc func(spec, cluster crclient.Object) (crclient.Object, error)

func defaultUpdateFunc(spec, cluster crclient.Object) (crclient.Object, error) {
	if cluster != nil {
		spec.SetResourceVersion(cluster.GetResourceVersion())
	}
	return spec, nil
}

func serviceUpdateFunc(spec, cluster crclient.Object) (crclient.Object, error) {
	if cluster == nil {
		return nil, errors.New("updating a service requires the cluster instance")
	}
	specService := spec.DeepCopyObject().(*corev1.Service)
	clusterService := cluster.(*corev1.Service)
	specService.ResourceVersion = clusterService.ResourceVersion
	specService.Spec.ClusterIP = clusterService.Spec.ClusterIP
	return specService, nil
}

func serviceAccountUpdateFunc(spec, cluster crclient.Object) (crclient.Object, error) {
	if cluster == nil {
		// May occur if ServiceAccount is not cached by the operator
		return spec, nil
	}
	spec.SetResourceVersion(cluster.GetResourceVersion())
	ownerrefs := spec.GetOwnerReferences()
	for _, clusterOwnerref := range cluster.GetOwnerReferences() {
		if !containsOwnerRef(clusterOwnerref, ownerrefs) {
			ownerrefs = append(ownerrefs, clusterOwnerref)
		}
	}
	spec.SetOwnerReferences(ownerrefs)
	return spec, nil
}

func getUpdateFunc(obj crclient.Object) updateFunc {
	objType := reflect.TypeOf(obj).Elem()
	switch objType {
	case reflect.TypeOf(corev1.Service{}):
		return serviceUpdateFunc
	case reflect.TypeOf(corev1.ServiceAccount{}):
		return serviceAccountUpdateFunc
	default:
		return defaultUpdateFunc
	}
}
