//
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
//

package controllers

import (
	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/constants"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// devworkspacePredicates filters incoming events to avoid unnecessary reconciles to failed workspaces.
// If a workspace failed and its spec is changed, we trigger reconciles to allow for fixing
// issues in the workspace spec.
var devworkspacePredicates = predicate.Funcs{
	CreateFunc: func(_ event.CreateEvent) bool { return true },
	DeleteFunc: func(_ event.DeleteEvent) bool { return true },
	UpdateFunc: func(ev event.UpdateEvent) bool {
		newObj, ok := ev.ObjectNew.(*dw.DevWorkspace)
		if !ok {
			return true
		}

		if newObj.Status.Phase != dw.DevWorkspaceStatusFailed {
			return true
		}

		oldObj, ok := ev.ObjectOld.(*dw.DevWorkspace)
		if !ok {
			// Should never happen
			return true
		}
		// always reconcile if resource is deleted
		if newObj.GetDeletionTimestamp() != nil {
			return true
		}

		// Trigger a reconcile on failed workspaces if spec is updated.
		return !equality.Semantic.DeepEqual(oldObj.Spec, newObj.Spec)
	},
	GenericFunc: func(_ event.GenericEvent) bool { return true },
}

var podPredicates = predicate.Funcs{
	UpdateFunc: func(ev event.UpdateEvent) bool {
		newObj, ok := ev.ObjectNew.(*corev1.Pod)
		if !ok {
			//that's no Pod. Let other predicates decide
			return true
		}

		oldObj, ok := ev.ObjectOld.(*corev1.Pod)
		if !ok {
			// Should never happen
			return true
		}

		// If the devworkspace label does not exist, do no reconcile
		if _, ok = newObj.Labels[constants.DevWorkspaceNameLabel]; !ok {
			return false
		}

		// Trigger a devworkspace reconcile if related pod status is updated.
		if equality.Semantic.DeepEqual(oldObj.Status, newObj.Status) {
			return false
		}

		return true
	},
}

var automountPredicates = predicate.Funcs{
	CreateFunc: func(ev event.CreateEvent) bool {
		return objectIsAutomountResource(ev.Object)
	},
	DeleteFunc: func(ev event.DeleteEvent) bool {
		return objectIsAutomountResource(ev.Object)
	},
	UpdateFunc: func(ev event.UpdateEvent) bool {
		return objectIsAutomountResource(ev.ObjectNew)
	},
	GenericFunc: func(_ event.GenericEvent) bool { return false },
}

func objectIsAutomountResource(obj client.Object) bool {
	labels := obj.GetLabels()
	switch {
	case labels[constants.DevWorkspaceMountLabel] == "true",
		labels[constants.DevWorkspaceGitCredentialLabel] == "true",
		labels[constants.DevWorkspaceGitTLSLabel] == "true",
		labels[constants.DevWorkspacePullSecretLabel] == "true":
		return true
	default:
		return false
	}

}
