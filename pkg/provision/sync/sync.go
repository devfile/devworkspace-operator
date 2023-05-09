// Copyright (c) 2019-2023 Red Hat, Inc.
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
	"fmt"
	"reflect"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// IsRecognizedObject returns whether the provided object kind is recognized by the sync package to support updating
// existing objects on the cluster. If the object is not recognized, passing it to SyncObjectWithCluster will fail.
// Instead, SyncUnrecognizedObjectWithCluster should be used.
func IsRecognizedObject(specObj crclient.Object) bool {
	objType := reflect.TypeOf(specObj).Elem()
	_, ok := diffFuncs[objType]
	return ok
}

// SyncObjectWithCluster synchronises the state of specObj to the cluster, creating or updating the cluster object
// as required. If specObj is in sync with the cluster, returns the object as it exists on the cluster. Returns a
// NotInSyncError if an update is required, UnrecoverableSyncError if object provided is invalid, or generic error
// if an unexpected error is encountered
func SyncObjectWithCluster(specObj crclient.Object, api ClusterAPI) (crclient.Object, error) {
	objType := reflect.TypeOf(specObj).Elem()
	clusterObj := reflect.New(objType).Interface().(crclient.Object)

	err := api.Client.Get(api.Ctx, types.NamespacedName{Name: specObj.GetName(), Namespace: specObj.GetNamespace()}, clusterObj)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, createObjectGeneric(specObj, api)
		}
		return nil, err
	}

	if !isMutableObject(specObj) { // TODO: we could still update labels here, or treat a need to update as a fatal error
		return clusterObj, nil
	}

	diffFunc := diffFuncs[objType]
	if diffFunc == nil {
		return nil, &UnrecoverableSyncError{fmt.Errorf("attempting to sync unrecognized object %s", objType)}
	}
	shouldDelete, shouldUpdate := diffFunc(specObj, clusterObj)
	if shouldDelete {
		printDiff(specObj, clusterObj, api.Logger)
		err := api.Client.Delete(api.Ctx, specObj)
		if err != nil {
			return nil, err
		}
		api.Logger.Info("Deleted object", "kind", objType.String(), "name", specObj.GetName())
		return nil, NewNotInSync(specObj, DeletedObjectReason)
	}
	if shouldUpdate {
		printDiff(specObj, clusterObj, api.Logger)
		return nil, updateObjectGeneric(specObj, clusterObj, api)
	}
	return clusterObj, nil
}

// SyncUnrecognizedObjectWithCluster allows syncing objects not supported by SyncObjectWithCluster. As there is
// no generic way of deciding if an object needs to be updated, a WarningError is returned if the object exists
// on the cluster. The only object updating performed by this function is to ensure labels/annotations and
// ownerReferences in specObj are synced to the cluster.
// The reason arbitrary updates are not supported is 1) certain objects have defaulted fields that can always
// trigger naive diff checks (causing reconciles to get stuck), and 2) it's unknown which fields are unmodifiable,
// (e.g. services must keep ClusterIP once set; pod fields cannot be changed after creation)
func SyncUnrecognizedObjectWithCluster(specObj crclient.Object, api ClusterAPI) (crclient.Object, error) {
	objType := reflect.TypeOf(specObj).Elem()
	clusterObj := reflect.New(objType).Interface().(crclient.Object)

	err := api.Client.Get(api.Ctx, types.NamespacedName{Name: specObj.GetName(), Namespace: specObj.GetNamespace()}, clusterObj)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, createObjectGeneric(specObj, api)
		}
		return nil, err
	}
	update, delete := unrecognizedObjectDiffFunc(specObj, clusterObj)
	if update {
		toUpdate, err := unrecognizedObjectUpdateFunc(specObj, clusterObj)
		if err != nil {
			return nil, &UnrecoverableSyncError{err}
		}
		err = api.Client.Update(api.Ctx, toUpdate)
		switch {
		case err == nil:
			api.Logger.Info("Updated object", "kind", reflect.TypeOf(specObj).Elem().String(), "name", specObj.GetName())
			return nil, NewNotInSync(specObj, UpdatedObjectReason)
		case k8sErrors.IsConflict(err):
			return nil, NewNotInSync(specObj, NeedRetryReason)
		case k8sErrors.IsInvalid(err), k8sErrors.IsForbidden(err):
			return nil, &UnrecoverableSyncError{err}
		default:
			return nil, err
		}
	}
	if delete {
		if err := api.Client.Delete(api.Ctx, clusterObj); err != nil {
			if k8sErrors.IsForbidden(err) {
				return nil, &UnrecoverableSyncError{fmt.Errorf("failed to delete object %s: %w", clusterObj.GetName(), err)}
			}
			return nil, err
		}
		api.Logger.Info("Deleted object", "kind", reflect.TypeOf(specObj).Elem().String(), "name", specObj.GetName())
		return nil, NewNotInSync(specObj, DeletedObjectReason)
	}
	kind := specObj.GetObjectKind().GroupVersionKind().GroupKind().String()
	return clusterObj, &WarningError{
		Message: fmt.Sprintf("Unrecognized object kind %s. Object will not be updated on cluster", kind),
	}
}

func createObjectGeneric(specObj crclient.Object, api ClusterAPI) error {
	err := api.Client.Create(api.Ctx, specObj)
	switch {
	case err == nil:
		api.Logger.Info("Created object", "kind", reflect.TypeOf(specObj).Elem().String(), "name", specObj.GetName())
		return NewNotInSync(specObj, CreatedObjectReason)
	case k8sErrors.IsAlreadyExists(err):
		// Need to try to update the object to address an edge case where removing a labelselector
		// results in the object not being tracked by the controller's cache.
		return updateObjectGeneric(specObj, nil, api)
	case k8sErrors.IsInvalid(err), k8sErrors.IsForbidden(err):
		return &UnrecoverableSyncError{err}
	default:
		return err
	}
}

func updateObjectGeneric(specObj, clusterObj crclient.Object, api ClusterAPI) error {
	updateFunc := getUpdateFunc(specObj)
	updatedObj, err := updateFunc(specObj, clusterObj)
	if err != nil {
		if err := api.Client.Delete(api.Ctx, specObj); err != nil {
			return err
		}
		api.Logger.Info("Deleted object", "kind", reflect.TypeOf(specObj).Elem().String(), "name", specObj.GetName())
		return NewNotInSync(specObj, DeletedObjectReason)
	}

	err = api.Client.Update(api.Ctx, updatedObj)
	switch {
	case err == nil:
		api.Logger.Info("Updated object", "kind", reflect.TypeOf(specObj).Elem().String(), "name", specObj.GetName())
		return NewNotInSync(specObj, UpdatedObjectReason)
	case k8sErrors.IsConflict(err), k8sErrors.IsNotFound(err):
		// Need to catch IsNotFound here because we attempt to update when creation fails with AlreadyExists
		return NewNotInSync(specObj, NeedRetryReason)
	case k8sErrors.IsInvalid(err), k8sErrors.IsForbidden(err):
		return &UnrecoverableSyncError{err}
	default:
		return err
	}
}

func isMutableObject(obj crclient.Object) bool {
	switch obj.(type) {
	case *corev1.PersistentVolumeClaim:
		return false
	default:
		return true
	}
}

func printDiff(specObj, clusterObj crclient.Object, log logr.Logger) {
	if config.IsSetUp() && config.ExperimentalFeaturesEnabled() {
		var diffOpts cmp.Options
		switch specObj.(type) {
		case *rbacv1.Role:
			diffOpts = roleDiffOpts
		case *rbacv1.RoleBinding:
			diffOpts = rolebindingDiffOpts
		case *appsv1.Deployment:
			diffOpts = deploymentDiffOpts
		case *corev1.Pod:
			diffOpts = podDiffOpts
		case *corev1.ConfigMap:
			diffOpts = configmapDiffOpts
		case *v1alpha1.DevWorkspaceRouting:
			diffOpts = routingDiffOpts
		case *networkingv1.Ingress:
			diffOpts = ingressDiffOpts
		case *routev1.Route:
			diffOpts = routeDiffOpts
		case *corev1.Secret:
			log.Info(fmt.Sprintf("Diff: secret %s data upated", specObj.GetName()))
			return
		default:
			diffOpts = nil
		}

		log.Info(fmt.Sprintf("Diff: %s", cmp.Diff(specObj, clusterObj, diffOpts)))
	}
}
