// Copyright (c) 2019-2025 Red Hat, Inc.
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

package controllers

import (
	"context"
	"fmt"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	wkspConfig "github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Mapping the pod to the devworkspace
func dwRelatedPodsHandler(ctx context.Context, obj client.Object) []reconcile.Request {
	labels := obj.GetLabels()
	if _, ok := labels[constants.DevWorkspaceNameLabel]; !ok {
		return []reconcile.Request{}
	}

	//If the dewworkspace label does not exist, do no reconcile
	if _, ok := labels[constants.DevWorkspaceIDLabel]; !ok {
		return []reconcile.Request{}
	}

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      labels[constants.DevWorkspaceNameLabel],
				Namespace: obj.GetNamespace(),
			},
		},
	}
}

func (r *DevWorkspaceReconciler) dwPVCHandler(ctx context.Context, obj client.Object) []reconcile.Request {
	if obj.GetDeletionTimestamp() == nil {
		// Do not reconcile unless PVC is being deleted.
		return []reconcile.Request{}
	}

	// Check if PVC is owned by a DevWorkspace (per-workspace storage case)
	// TODO: Ensure all new and existing PVC's get the `controller.devfile.io/devworkspace_pvc_type` label.
	// See: https://github.com/devfile/devworkspace-operator/issues/1250
	for _, ownerref := range obj.GetOwnerReferences() {
		if ownerref.Kind != "DevWorkspace" {
			continue
		}
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      ownerref.Name,
					Namespace: obj.GetNamespace(),
				},
			},
		}
	}

	pvcLabel := obj.GetLabels()[constants.DevWorkspacePVCTypeLabel]

	// TODO: This check is for legacy reasons as existing PVCs might not have the `controller.devfile.io/devworkspace_pvc_type` label.
	// Remove once https://github.com/devfile/devworkspace-operator/issues/1250 is resolved
	if pvcLabel == "" {
		if obj.GetName() != wkspConfig.GetGlobalConfig().Workspace.PVCName {
			// No need to reconcile if PVC doesn't have a PVC type label
			// and it doesn't have a name of PVC from global config.
			return []reconcile.Request{}
		}
	}

	dwList := &dw.DevWorkspaceList{}
	if err := r.Client.List(context.Background(), dwList, &client.ListOptions{Namespace: obj.GetNamespace()}); err != nil {
		return []reconcile.Request{}
	}
	var reconciles []reconcile.Request

	for _, workspace := range dwList.Items {
		storageType := workspace.Spec.Template.Attributes.GetString(constants.DevWorkspaceStorageTypeAttribute, nil)
		if storageType == constants.CommonStorageClassType || storageType == constants.PerUserStorageClassType || storageType == "" {

			// Determine workspaces to reconcile that use the current common PVC.
			// Workspaces can either use the common PVC where the PVC name
			// is coming from the global config, or from an external config the workspace might use
			workspacePVCName := wkspConfig.GetGlobalConfig().Workspace.PVCName

			if workspace.Spec.Template.Attributes.Exists(constants.ExternalDevWorkspaceConfiguration) {
				externalConfig, err := wkspConfig.ResolveConfigForWorkspace(&workspace, r.Client)
				if err != nil {
					r.Log.Info(fmt.Sprintf("Couldn't resolve PVC name for workspace '%s' in namespace '%s', using PVC name '%s' from global config instead: %s.", workspace.Name, workspace.Namespace, workspacePVCName, err.Error()))
				} else {
					workspacePVCName = externalConfig.Workspace.PVCName
				}
			}
			if obj.GetName() == workspacePVCName {
				reconciles = append(reconciles, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      workspace.GetName(),
						Namespace: workspace.GetNamespace(),
					},
				})
			}
		}
	}
	return reconciles
}

func (r *DevWorkspaceReconciler) runningWorkspacesHandler(ctx context.Context, obj client.Object) []reconcile.Request {
	dwList := &dw.DevWorkspaceList{}
	if err := r.Client.List(context.Background(), dwList, &client.ListOptions{Namespace: obj.GetNamespace()}); err != nil {
		return []reconcile.Request{}
	}
	var reconciles []reconcile.Request
	for _, workspace := range dwList.Items {
		// Queue reconciles for any started workspaces to make sure they pick up new object
		if workspace.Spec.Started {
			reconciles = append(reconciles, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      workspace.GetName(),
					Namespace: workspace.GetNamespace(),
				},
			})
		}
	}
	return reconciles
}
