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

package cache

import (
	"github.com/devfile/devworkspace-operator/pkg/constants"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

// GetCacheFunc returns a new cache function that restricts the cluster items we store in the manager's
// internal cache. This is required because the controller watches a lot of resource types, which can
// result in very high memory usage on large clusters (e.g. clusters with tens of thousands of secrets).
func GetCacheFunc() (cache.NewCacheFunc, error) {
	devworkspaceObjectSelector, err := labels.Parse(constants.DevWorkspaceIDLabel)
	if err != nil {
		return nil, err
	}

	selectors := cache.SelectorsByObject{
		&appsv1.Deployment{}: {
			Label: devworkspaceObjectSelector,
		},
		&corev1.Pod{}: {
			Label: devworkspaceObjectSelector,
		},
		&batchv1.Job{}: {
			Label: devworkspaceObjectSelector,
		},
		&corev1.ServiceAccount{}: {
			Label: devworkspaceObjectSelector,
		},
		&corev1.Service{}: {
			Label: devworkspaceObjectSelector,
		},
		&networkingv1.Ingress{}: {
			Label: devworkspaceObjectSelector,
		},
		&routev1.Route{}: {
			Label: devworkspaceObjectSelector,
		},
	}

	return cache.BuilderWithOptions(cache.Options{
		SelectorsByObject: selectors,
	}), nil
}
