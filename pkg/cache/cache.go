// Copyright (c) 2019-2025 Red Hat, Inc.
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
	"fmt"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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

	// We have to treat secrets and configmaps separately since we need to support auto-mounting
	secretObjectSelector, err := labels.Parse(fmt.Sprintf("%s=true", constants.DevWorkspaceWatchSecretLabel))
	if err != nil {
		return nil, err
	}
	configmapObjectSelector, err := labels.Parse(fmt.Sprintf("%s=true", constants.DevWorkspaceWatchConfigMapLabel))
	if err != nil {
		return nil, err
	}
	cronJobObjectSelector, err := labels.Parse(fmt.Sprintf("%s=true", constants.DevWorkspaceWatchCronJobLabel))
	if err != nil {
		return nil, err
	}
	rbacObjectSelector, err := labels.Parse("controller.devfile.io/workspace-rbac=true")
	if err != nil {
		return nil, err
	}

	selectors := map[client.Object]cache.ByObject{
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
		&batchv1.CronJob{}: {
			Label: cronJobObjectSelector,
		},
		&corev1.ConfigMap{}: {
			Label: configmapObjectSelector,
		},
		&corev1.Secret{}: {
			Label: secretObjectSelector,
		},
		&rbacv1.Role{}: {
			Label: rbacObjectSelector,
		},
		&rbacv1.RoleBinding{}: {
			Label: rbacObjectSelector,
		},
	}

	if infrastructure.IsOpenShift() {
		selectors[&routev1.Route{}] = cache.ByObject{Label: devworkspaceObjectSelector}
	}

	return func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
		opts.ByObject = selectors
		return cache.New(config, opts)
	}, nil
}

// GetWebhooksCacheFunc returns a new cache function that restricts the cluster items we store in the webhook
// server's internal cache. This avoids issues where the webhook server's memory usage scales with the number
// of objects on the cluster, potentially causing out of memory errors in large clusters.
func GetWebhooksCacheFunc(namespace string) (cache.NewCacheFunc, error) {
	// The webhooks server needs to read pods to validate pods/exec requests. These pods must have the DevWorkspace ID and restricted
	// access labels (other pods are automatically approved)
	devworkspaceObjectSelector, err := labels.Parse(constants.DevWorkspaceIDLabel)
	if err != nil {
		return nil, err
	}

	selectors := map[client.Object]cache.ByObject{
		&corev1.Pod{}: {
			Label: devworkspaceObjectSelector,
		},
	}

	return func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
		opts.ByObject = selectors
		opts.DefaultNamespaces = map[string]cache.Config{
			namespace: {},
		}
		return cache.New(config, opts)
	}, nil
}
