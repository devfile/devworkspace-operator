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

package workspacerouting

import (
	"errors"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func getRoutingPredicatesForSolverFunc(getSolver WorkspaceRoutingSolverFunc) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(ev event.CreateEvent) bool {
			obj, ok := ev.Object.(*controllerv1alpha1.WorkspaceRouting)
			if !ok {
				// If object is not a WorkspaceRouting, it must be a service/ingress/route related to the workspace
				// The safe choice here is to trigger a reconcile to ensure that all resources are in sync; it's the job
				// of the controller to ignore WorkspaceRoutings for other routing classes.
				return true
			}
			if _, err := getSolver(obj.Spec.RoutingClass); errors.Is(err, RoutingNotSupported) {
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
			newObj, ok := ev.ObjectNew.(*controllerv1alpha1.WorkspaceRouting)
			if !ok {
				// If object is not a WorkspaceRouting, it must be a service/ingress/route related to the workspace
				// The safe choice here is to trigger a reconcile to ensure that all resources are in sync; it's the job
				// of the controller to ignore WorkspaceRoutings for other routing classes.
				return true
			}
			if _, err := getSolver(newObj.Spec.RoutingClass); errors.Is(err, RoutingNotSupported) {
				return false
			}
			return true
		},
		GenericFunc: func(ev event.GenericEvent) bool {
			obj, ok := ev.Object.(*controllerv1alpha1.WorkspaceRouting)
			if !ok {
				return true
			}
			if _, err := getSolver(obj.Spec.RoutingClass); errors.Is(err, RoutingNotSupported) {
				return false
			}
			return true
		},
	}
}
