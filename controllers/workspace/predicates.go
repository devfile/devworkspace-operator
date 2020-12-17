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

package controllers

import (
	"github.com/devfile/api/pkg/apis/workspaces/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var predicates = predicate.Funcs{
	CreateFunc: func(_ event.CreateEvent) bool { return true },
	DeleteFunc: func(_ event.DeleteEvent) bool { return true },
	UpdateFunc: func(ev event.UpdateEvent) bool {
		newObj, ok := ev.ObjectNew.(*v1alpha2.DevWorkspace)
		if !ok {
			return true
		}
		if newObj.Status.Phase != v1alpha2.WorkspaceStatusFailed {
			return true
		}
		oldObj, ok := ev.ObjectOld.(*v1alpha2.DevWorkspace)
		if !ok {
			// Should never happen
			return true
		}
		// Trigger a reconcile on failed workspaces if routingClass is updated.
		return oldObj.Spec.RoutingClass != newObj.Spec.RoutingClass
	},
	GenericFunc: func(_ event.GenericEvent) bool { return true },
}
