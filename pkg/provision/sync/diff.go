// Copyright (c) 2019-2022 Red Hat, Inc.
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
	"sort"
	"strings"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/google/go-cmp/cmp"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// diffFunc represents a function that compares a spec object against the corresponding cluster object and
// returns whether the object should be deleted or updated.
type diffFunc func(spec crclient.Object, cluster crclient.Object) (delete, update bool)

var diffFuncs = map[reflect.Type]diffFunc{
	reflect.TypeOf(rbacv1.Role{}):                  allDiffFuncs(labelsAndAnnotationsDiffFunc, basicDiffFunc(roleDiffOpts)),
	reflect.TypeOf(rbacv1.RoleBinding{}):           allDiffFuncs(labelsAndAnnotationsDiffFunc, basicDiffFunc(rolebindingDiffOpts)),
	reflect.TypeOf(corev1.ServiceAccount{}):        allDiffFuncs(labelsAndAnnotationsDiffFunc, ownerrefsDiffFunc),
	reflect.TypeOf(appsv1.Deployment{}):            allDiffFuncs(deploymentDiffFunc, labelsAndAnnotationsDiffFunc, basicDiffFunc(deploymentDiffOpts)),
	reflect.TypeOf(corev1.ConfigMap{}):             allDiffFuncs(labelsAndAnnotationsDiffFunc, basicDiffFunc(configmapDiffOpts)),
	reflect.TypeOf(corev1.Secret{}):                allDiffFuncs(labelsAndAnnotationsDiffFunc, basicDiffFunc(secretDiffOpts)),
	reflect.TypeOf(v1alpha1.DevWorkspaceRouting{}): allDiffFuncs(routingDiffFunc, labelsAndAnnotationsDiffFunc, basicDiffFunc(routingDiffOpts)),
	reflect.TypeOf(batchv1.Job{}):                  allDiffFuncs(labelsAndAnnotationsDiffFunc, jobDiffFunc),
	reflect.TypeOf(corev1.Service{}):               allDiffFuncs(labelsAndAnnotationsDiffFunc, serviceDiffFunc),
	reflect.TypeOf(networkingv1.Ingress{}):         allDiffFuncs(labelsAndAnnotationsDiffFunc, basicDiffFunc(ingressDiffOpts)),
	reflect.TypeOf(routev1.Route{}):                allDiffFuncs(labelsAndAnnotationsDiffFunc, basicDiffFunc(routeDiffOpts)),
}

// basicDiffFunc returns a diffFunc that specifies an object needs an update if cmp.Equal fails
func basicDiffFunc(diffOpt cmp.Options) diffFunc {
	return func(spec, cluster crclient.Object) (delete, update bool) {
		return false, !cmp.Equal(spec, cluster, diffOpt)
	}
}

// labelsAndAnnotationsDiffFunc requires an object to be updated if any label or annotation present in the spec
// object is not present in the cluster object.
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

func ownerrefsDiffFunc(spec, cluster crclient.Object) (delete, update bool) {
	clusterRefs := cluster.GetOwnerReferences()
	for _, ownerref := range spec.GetOwnerReferences() {
		if !containsOwnerRef(ownerref, clusterRefs) {
			return false, true
		}
	}
	return false, false
}

// allDiffFuncs represents an 'and' condition across specified diffFuncs. Functions are checked in provided order,
// returning the result of the first function to require an update/deletion.
func allDiffFuncs(funcs ...diffFunc) diffFunc {
	return func(spec, cluster crclient.Object) (delete, update bool) {
		// Need to check each function in case one requires the object to be deleted
		anyDelete, anyUpdate := false, false
		for _, df := range funcs {
			shouldDelete, shouldUpdate := df(spec, cluster)
			anyDelete = anyDelete || shouldDelete
			anyUpdate = anyUpdate || shouldUpdate
		}
		return anyDelete, anyUpdate
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

func jobDiffFunc(spec, cluster crclient.Object) (delete, update bool) {
	specJob := spec.(*batchv1.Job)
	clusterJob := cluster.(*batchv1.Job)
	// TODO: previously, this delete was specified with a background deletion policy, which is currently unsupported.
	return !equality.Semantic.DeepDerivative(specJob.Spec, clusterJob.Spec), false
}

func serviceDiffFunc(spec, cluster crclient.Object) (delete, update bool) {
	specService := spec.(*corev1.Service)
	clusterService := cluster.(*corev1.Service)
	specCopy := specService.DeepCopy()
	clusterCopy := clusterService.DeepCopy()
	if !cmp.Equal(specCopy.Spec.Selector, clusterCopy.Spec.Selector) {
		return false, true
	}
	// Function that takes a slice of servicePorts and returns the appropriate comparison
	// function to pass to sort.Slice() for that slice of servicePorts.
	servicePortSorter := func(servicePorts []corev1.ServicePort) func(i, j int) bool {
		return func(i, j int) bool {
			return strings.Compare(servicePorts[i].Name, servicePorts[j].Name) > 0
		}
	}
	sort.Slice(specCopy.Spec.Ports, servicePortSorter(specCopy.Spec.Ports))
	sort.Slice(clusterCopy.Spec.Ports, servicePortSorter(clusterCopy.Spec.Ports))
	if !cmp.Equal(specCopy.Spec.Ports, clusterCopy.Spec.Ports) {
		return false, true
	}
	return false, specCopy.Spec.Type != clusterCopy.Spec.Type
}

func containsOwnerRef(toCheck metav1.OwnerReference, listRefs []metav1.OwnerReference) bool {
	boolPtrsEqual := func(a, b *bool) bool {
		// If either is nil, assume check other is nil or false; otherwise, compare actual values
		switch {
		case a == nil:
			return b == nil || !*b
		case b == nil:
			return a == nil || !*a
		default:
			return *a == *b
		}
	}
	for _, ref := range listRefs {
		if toCheck.Kind == ref.Kind &&
			toCheck.Name == ref.Name &&
			toCheck.UID == ref.UID &&
			boolPtrsEqual(toCheck.Controller, ref.Controller) {
			return true
		}
	}
	return false
}
