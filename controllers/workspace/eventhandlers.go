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
	// No need to reconcile PVC, if it doesn't have a label specifying its type
	pvcLabel, ok := obj.GetLabels()[constants.DevWorkspacePVCTypeLabel]
	if !ok {
		return []reconcile.Request{}
	}

	if pvcLabel == constants.PerWorkspaceStorageClassType {
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
	}

	if pvcLabel == constants.CommonStorageClassType || pvcLabel == constants.PerUserStorageClassType || pvcLabel == "" {
		dwList := &dw.DevWorkspaceList{}
		if err := r.Client.List(context.Background(), dwList, &client.ListOptions{Namespace: obj.GetNamespace()}); err != nil {
			return []reconcile.Request{}
		}
		var reconciles []reconcile.Request
		for _, workspace := range dwList.Items {
			// determin if PVC belongs to the workspace that has to be reconcile by its name
			// which comes from the DW operator global, or external config that may be defined
			// for this workspace
			var workspacePvcName string;
			if (workspace.Spec.Template.Attributes.Exists(constants.ExternalDevWorkspaceConfiguration)) {
				externalConfig, _ := wkspConfig.ResolveConfigForWorkspace(&workspace, r.Client)
				workspacePvcName = externalConfig.Workspace.PVCName
			}
			if len(workspacePvcName) == 0 {
				workspacePvcName = wkspConfig.GetGlobalConfig().Workspace.PVCName
			}

			storageType := workspace.Spec.Template.Attributes.GetString(constants.DevWorkspaceStorageTypeAttribute, nil)
			if storageType == constants.CommonStorageClassType || storageType == constants.PerUserStorageClassType || storageType == "" {
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
	// shouldn't reach here
	return []reconcile.Request{} 	
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
