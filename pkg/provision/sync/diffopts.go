// Copyright (c) 2019-2023 Red Hat, Inc.
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

package sync

import (
	"strings"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	routeV1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

var roleDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(rbacv1.Role{}, "TypeMeta", "ObjectMeta"),
}

var rolebindingDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(rbacv1.RoleBinding{}, "TypeMeta", "ObjectMeta"),
	cmpopts.IgnoreFields(rbacv1.RoleRef{}, "APIGroup"),
	cmpopts.IgnoreFields(rbacv1.Subject{}, "APIGroup"),
}

var deploymentDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(appsv1.Deployment{}, "TypeMeta", "ObjectMeta", "Status"),
	cmpopts.IgnoreFields(appsv1.DeploymentSpec{}, "RevisionHistoryLimit", "ProgressDeadlineSeconds"),
	cmpopts.IgnoreFields(corev1.PodSpec{}, "DNSPolicy", "SchedulerName", "DeprecatedServiceAccount"),
	cmpopts.IgnoreFields(corev1.Container{}, "TerminationMessagePath", "TerminationMessagePolicy", "ImagePullPolicy"),
	cmpopts.SortSlices(func(a, b corev1.Container) bool {
		return strings.Compare(a.Name, b.Name) > 0
	}),
	cmpopts.SortSlices(func(a, b corev1.Volume) bool {
		return strings.Compare(a.Name, b.Name) > 0
	}),
	cmpopts.SortSlices(func(a, b corev1.VolumeMount) bool {
		switch {
		case a.Name != b.Name:
			return strings.Compare(a.Name, b.Name) > 0
		case a.MountPath != b.MountPath:
			return strings.Compare(a.MountPath, b.MountPath) > 0
		case a.SubPath != b.SubPath:
			return strings.Compare(a.SubPath, b.SubPath) > 0
		default:
			// If mountPath + subPath match, the deployment is invalid, so this cannot happen.
			return false
		}
	}),
	cmpopts.SortSlices(func(a, b corev1.EnvFromSource) bool {
		return strings.Compare(getNameFromEnvFrom(a), getNameFromEnvFrom(b)) > 0
	}),
}

var podDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(corev1.Pod{}, "TypeMeta", "ObjectMeta", "Status"),
}

var configmapDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta", "ObjectMeta"),
}

var secretDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(corev1.Secret{}, "TypeMeta", "ObjectMeta"),
}

var routingDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(v1alpha1.DevWorkspaceRouting{}, "ObjectMeta", "TypeMeta", "Status"),
}

var routeDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(routeV1.Route{}, "TypeMeta", "ObjectMeta", "Status"),
	cmpopts.IgnoreFields(routeV1.RouteSpec{}, "WildcardPolicy", "Host"),
	cmpopts.IgnoreFields(routeV1.RouteTargetReference{}, "Weight"),
}

var ingressDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(networkingv1.Ingress{}, "TypeMeta", "ObjectMeta", "Status"),
	cmpopts.IgnoreFields(networkingv1.HTTPIngressPath{}, "PathType"),
}

func getNameFromEnvFrom(source corev1.EnvFromSource) string {
	switch {
	case source.ConfigMapRef != nil:
		return source.ConfigMapRef.Name
	case source.SecretRef != nil:
		return source.SecretRef.Name
	default:
		return ""
	}
}
