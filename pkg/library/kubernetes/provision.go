// Copyright (c) 2019-2024 Red Hat, Inc.
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

package kubernetes

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// HandleKubernetesComponents processes the Kubernetes and OpenShift components of a DevWorkspace,
// creating/updating objects on the cluster. This function does not verify if the workspace owner
// has the correct permissions to create/update/delete these objects and instead assumes the
// workspace owner has all applicable RBAC permissions.
// Only Kubernetes/OpenShift components that are inlined are supported; components that define
// a URI will cause a WarningError to be returned
func HandleKubernetesComponents(workspace *common.DevWorkspaceWithConfig, api sync.ClusterAPI) error {
	kubeComponents, warnings, err := filterForKubeLikeComponents(workspace.Spec.Template.Components)
	if err != nil {
		return err
	}
	if len(kubeComponents) == 0 {
		if len(warnings) > 0 {
			return &dwerrors.WarningError{
				Message: fmt.Sprintf("Ignored components that use unsupported features: %s", strings.Join(warnings, ", ")),
			}
		}
		return nil
	}
	for _, component := range kubeComponents {
		// Ignore error as we filtered list above
		k8sLikeComponent, _ := getK8sLikeComponent(component)
		obj, err := deserializeToObject([]byte(k8sLikeComponent.Inlined), api)
		if err != nil {
			return &dwerrors.FailError{Message: fmt.Sprintf("could not process component %s", component.Name), Err: err}
		}
		if err := addMetadata(obj, workspace, api); err != nil {
			return &dwerrors.RetryError{Message: fmt.Sprintf("failed to add ownerref for component %s", component.Name), Err: err}
		}
		if err := checkForExistingObject(obj, api); err != nil {
			return &dwerrors.FailError{Message: fmt.Sprintf("could not process component %s", component.Name), Err: err}
		}
		var syncErr error
		if sync.IsRecognizedObject(obj) {
			_, syncErr = sync.SyncObjectWithCluster(obj, api)
		} else {
			_, syncErr = sync.SyncUnrecognizedObjectWithCluster(obj, api)
		}
		if syncErr != nil {
			return dwerrors.WrapSyncError(syncErr)
		}
	}
	if len(warnings) > 0 {
		return &dwerrors.WarningError{
			Message: fmt.Sprintf("Ignored components that use unsupported features: %s", strings.Join(warnings, ", ")),
		}
	}
	return nil
}

func checkForExistingObject(obj client.Object, api sync.ClusterAPI) error {
	objType := reflect.TypeOf(obj).Elem()
	clusterObj := reflect.New(objType).Interface().(crclient.Object)
	err := api.Client.Get(api.Ctx, client.ObjectKey{Name: obj.GetName(), Namespace: obj.GetNamespace()}, clusterObj)
	switch {
	case err == nil:
		break
	case k8sErrors.IsNotFound(err):
		// Object does not exist yet; safe to create
		return nil
	default:
		return err
	}
	existingWorkspaceID := clusterObj.GetLabels()[constants.DevWorkspaceIDLabel]
	expectedWorkspaceID := obj.GetLabels()[constants.DevWorkspaceIDLabel]
	if existingWorkspaceID != expectedWorkspaceID {
		return fmt.Errorf("object %s exists and is not owned by this workspace", obj.GetName())
	}
	if err := checkOwnerrefs(clusterObj.GetOwnerReferences(), obj.GetOwnerReferences()); err != nil {
		return fmt.Errorf("object %s exists and is not owned by this workspace", obj.GetName())
	}
	return nil
}

func addMetadata(obj client.Object, workspace *common.DevWorkspaceWithConfig, api sync.ClusterAPI) error {
	obj.SetNamespace(workspace.Namespace)
	if err := controllerutil.SetOwnerReference(workspace.DevWorkspace, obj, api.Scheme); err != nil {
		return err
	}
	newLabels := map[string]string{}
	for k, v := range obj.GetLabels() {
		newLabels[k] = v
	}
	newLabels[constants.DevWorkspaceIDLabel] = workspace.Status.DevWorkspaceId
	newLabels[constants.DevWorkspaceCreatorLabel] = workspace.Labels[constants.DevWorkspaceCreatorLabel]
	if obj.GetObjectKind().GroupVersionKind().Kind == "Secret" {
		newLabels[constants.DevWorkspaceWatchSecretLabel] = "true"
	}
	if obj.GetObjectKind().GroupVersionKind().Kind == "ConfigMap" {
		newLabels[constants.DevWorkspaceWatchConfigMapLabel] = "true"
	}
	obj.SetLabels(newLabels)
	return nil
}
