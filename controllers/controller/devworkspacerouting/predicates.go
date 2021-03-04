//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package devworkspacerouting

import (
	"github.com/devfile/devworkspace-operator/controllers/controller/devworkspacerouting/solvers"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func getRoutingPredicatesForSolverFunc(solverGetter solvers.RoutingSolverGetter) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(ev event.CreateEvent) bool {
			obj, ok := ev.Object.(*controllerv1alpha1.DevWorkspaceRouting)
			if !ok {
				// If object is not a DevWorkspaceRouting, it must be a service/ingress/route related to the workspace
				// The safe choice here is to trigger a reconcile to ensure that all resources are in sync; it's the job
				// of the controller to ignore DevWorkspaceRoutings for other routing classes.
				return true
			}
			if !solverGetter.HasSolver(obj.Spec.RoutingClass) {
				return false
			}
			return true
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			// Return true to ensure objects are recreated if needed, and that finalizers are
			// removed on deletion.
			return true
		},
		UpdateFunc: func(ev event.UpdateEvent) bool {
			newObj, ok := ev.ObjectNew.(*controllerv1alpha1.DevWorkspaceRouting)
			if !ok {
				// If object is not a DevWorkspaceRouting, it must be a service/ingress/route related to the workspace
				// The safe choice here is to trigger a reconcile to ensure that all resources are in sync; it's the job
				// of the controller to ignore DevWorkspaceRoutings for other routing classes.
				return true
			}
			if !solverGetter.HasSolver(newObj.Spec.RoutingClass) {
				// Future improvement: handle case where old object has a supported routingClass and new object does not
				// to allow for cleanup when routingClass is switched.
				return false
			}
			return true
		},
		GenericFunc: func(ev event.GenericEvent) bool {
			obj, ok := ev.Object.(*controllerv1alpha1.DevWorkspaceRouting)
			if !ok {
				return true
			}
			if !solverGetter.HasSolver(obj.Spec.RoutingClass) {
				return false
			}
			return true
		},
	}
}
