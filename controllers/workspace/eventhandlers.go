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

package controllers

import (
	"context"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	wkspConfig "github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Mapping the pod to the devworkspace
func dwRelatedPodsHandler(obj client.Object) []reconcile.Request {
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

func (r *DevWorkspaceReconciler) dwPVCHandler(obj client.Object) []reconcile.Request {
	if obj.GetDeletionTimestamp() == nil {
		return []reconcile.Request{}
	}
	// we can wrap the code below for handling per-workspace PVCs
	// in the if-block to check for presence of the respective label,
	// once we ensure that it is applied to all per-workspace PVCs
	// see comments to https://github.com/devfile/devworkspace-operator/pull/1233/files
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
	// No need to reconcile if PVC is being deleted, or it doesn't have a PVC type label.
	// However, since it is possible for PVCs to not have such label,
	// we will handle this PVC if it has a name that correspons with PVC name in global config
	// see comments to https://github.com/devfile/devworkspace-operator/pull/1233/files
	if pvcLabel != "" {
		if obj.GetName() != wkspConfig.GetGlobalConfig().Workspace.PVCName {
			return []reconcile.Request{}
		}
	}

	dwList := &dw.DevWorkspaceList{}
	if err := r.Client.List(context.Background(), dwList, &client.ListOptions{Namespace: obj.GetNamespace()}); err != nil {
		return []reconcile.Request{}
	}
	var reconciles []reconcile.Request

	for _, workspace := range dwList.Items {
		//Determine workspaces to reconcile that use the current common PVC.
		// Workspaces can either use the common PVC where the PVC name
		// is coming from the global config, or from an external config the workspace might use
		workspacePVCName := wkspConfig.GetGlobalConfig().Workspace.PVCName

		if workspace.Spec.Template.Attributes.Exists(constants.ExternalDevWorkspaceConfiguration) {
			externalConfig, err := wkspConfig.ResolveConfigForWorkspace(&workspace, r.Client)
			if err != nil {
				r.Log.Info("Couldn't fetch external config for workspace %s, using PVC Name from global config instead", err.Error())
			}
			storageType := workspace.Spec.Template.Attributes.GetString(constants.DevWorkspaceStorageTypeAttribute, nil)
			if storageType == constants.CommonStorageClassType || storageType == constants.PerUserStorageClassType {
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
	return reconciles
}

func (r *DevWorkspaceReconciler) runningWorkspacesHandler(obj client.Object) []reconcile.Request {
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
