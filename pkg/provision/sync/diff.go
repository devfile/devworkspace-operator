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

package sync

import (
	"reflect"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type diffFunc func(spec crclient.Object, cluster crclient.Object) (delete, update bool)

var diffFuncs = map[reflect.Type]diffFunc{
	reflect.TypeOf(rbacv1.Role{}):                  basicDiffFunc(roleDiffOpts),
	reflect.TypeOf(rbacv1.RoleBinding{}):           basicDiffFunc(rolebindingDiffOpts),
	reflect.TypeOf(corev1.ServiceAccount{}):        labelsAndAnnotationsDiffFunc,
	reflect.TypeOf(appsv1.Deployment{}):            allDiffFuncs(deploymentDiffFunc, basicDiffFunc(deploymentDiffOpts)),
	reflect.TypeOf(v1alpha1.DevWorkspaceRouting{}): allDiffFuncs(routingDiffFunc, labelsAndAnnotationsDiffFunc, basicDiffFunc(routingDiffOpts)),
}

func basicDiffFunc(diffOpt cmp.Options) diffFunc {
	return func(spec, cluster crclient.Object) (delete, update bool) {
		return false, !cmp.Equal(spec, cluster, diffOpt)
	}
}

func labelsAndAnnotationsDiffFunc(spec, cluster crclient.Object) (delete, update bool) {
	clusterAnnotations := cluster.GetAnnotations()
	for k, v := range spec.GetAnnotations() {
		if clusterAnnotations[k] != v {
			return false, true
		}
	}
	clusterLabels := cluster.GetLabels()
	for k, v := range spec.GetLabels() {
		if clusterLabels[k] != v {
			return false, true
		}
	}
	return false, false
}

func allDiffFuncs(funcs ...diffFunc) diffFunc {
	return func(spec, cluster crclient.Object) (delete, update bool) {
		for _, df := range funcs {
			shouldDelete, shouldUpdate := df(spec, cluster)
			if shouldDelete || shouldUpdate {
				return shouldDelete, shouldUpdate
			}
		}
		return false, false
	}
}

func deploymentDiffFunc(spec, cluster crclient.Object) (delete, update bool) {
	specDeploy := spec.(*appsv1.Deployment)
	clusterDeploy := cluster.(*appsv1.Deployment)
	if !cmp.Equal(specDeploy.Spec.Selector, clusterDeploy.Spec.Selector) {
		return true, false
	}
	return false, false
}

func routingDiffFunc(spec, cluster crclient.Object) (delete, update bool) {
	specRouting := spec.(*v1alpha1.DevWorkspaceRouting)
	clusterRouting := cluster.(*v1alpha1.DevWorkspaceRouting)
	if specRouting.Spec.RoutingClass != clusterRouting.Spec.RoutingClass {
		return true, false
	}
	return false, false
}
