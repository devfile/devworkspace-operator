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

package controllers

import (
	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func (r *DevWorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	maxConcurrentReconciles, err := config.GetMaxConcurrentReconciles()
	if err != nil {
		return err
	}

	var emptyMapper = func(obj client.Object) []reconcile.Request {
		return []reconcile.Request{}
	}

	var configWatcher builder.WatchesOption = builder.WithPredicates(config.Predicates())

	// TODO: Set up indexing https://book.kubebuilder.io/cronjob-tutorial/controller-implementation.html#setup
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}).
		For(&dw.DevWorkspace{}).
		// List DevWorkspaceTemplates as owned to enable updating workspaces when templates
		// are changed; this should be moved to whichever controller is responsible for flattening
		// DevWorkspaces
		Owns(&dw.DevWorkspaceTemplate{}).
		Owns(&appsv1.Deployment{}).
		Owns(&batchv1.Job{}).
		Owns(&controllerv1alpha1.DevWorkspaceRouting{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ServiceAccount{}).
		Watches(&source.Kind{Type: &corev1.Pod{}}, dwRelatedPodsHandler()).
		Watches(&source.Kind{Type: &controllerv1alpha1.DevWorkspaceOperatorConfig{}}, handler.EnqueueRequestsFromMapFunc(emptyMapper), configWatcher).
		WithEventFilter(predicates).
		WithEventFilter(podPredicates).
		Complete(r)
}

// Mapping the pod to the devworkspace
func dwRelatedPodsHandler() handler.EventHandler {
	podToDW := func(obj client.Object) []reconcile.Request {
		labels := obj.GetLabels()
		if _, ok := labels[constants.DevWorkspaceNameLabel]; !ok {
			return nil
		}

		//If the dewworkspace label does not exist, do no reconcile
		if _, ok := labels[constants.DevWorkspaceIDLabel]; !ok {
			return nil
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
	return handler.EnqueueRequestsFromMapFunc(podToDW)
}
