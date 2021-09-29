//
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
