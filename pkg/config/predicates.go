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

package config

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	dw "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
)

func Predicates() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(evt event.UpdateEvent) bool {
			if config, ok := evt.ObjectNew.(*dw.DevWorkspaceOperatorConfig); ok {
				syncConfigFrom(config)
			}
			return false
		},
		CreateFunc: func(evt event.CreateEvent) bool {
			if config, ok := evt.Object.(*dw.DevWorkspaceOperatorConfig); ok {
				syncConfigFrom(config)
			}
			return false
		},
		DeleteFunc: func(evt event.DeleteEvent) bool {
			if config, ok := evt.Object.(*dw.DevWorkspaceOperatorConfig); ok {
				if config.Name == OperatorConfigName && config.Namespace == configNamespace {
					restoreDefaultConfig()
				}
			}
			return false
		},
		GenericFunc: func(evt event.GenericEvent) bool {
			return false
		},
	}
}
