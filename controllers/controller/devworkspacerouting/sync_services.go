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
package devworkspacerouting

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/devfile/devworkspace-operator/pkg/constants"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
)

var serviceDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(corev1.Service{}, "TypeMeta", "ObjectMeta", "Status"),
	cmp.Comparer(func(x, y corev1.ServiceSpec) bool {
		xCopy := x.DeepCopy()
		yCopy := y.DeepCopy()
		if !cmp.Equal(xCopy.Selector, yCopy.Selector) {
			return false
		}
		// Function that takes a slice of servicePorts and returns the appropriate comparison
		// function to pass to sort.Slice() for that slice of servicePorts.
		servicePortSorter := func(servicePorts []corev1.ServicePort) func(i, j int) bool {
			return func(i, j int) bool {
				return strings.Compare(servicePorts[i].Name, servicePorts[j].Name) > 0
			}
		}
		sort.Slice(xCopy.Ports, servicePortSorter(xCopy.Ports))
		sort.Slice(yCopy.Ports, servicePortSorter(yCopy.Ports))
		if !cmp.Equal(xCopy.Ports, yCopy.Ports) {
			return false
		}
		return xCopy.Type == yCopy.Type
	}),
}

func (r *DevWorkspaceRoutingReconciler) syncServices(routing *controllerv1alpha1.DevWorkspaceRouting, specServices []corev1.Service) (ok bool, clusterServices []corev1.Service, err error) {
	servicesInSync := true

	clusterServices, err = r.getClusterServices(routing)
	if err != nil {
		return false, nil, err
	}

	toDelete := getServicesToDelete(clusterServices, specServices)
	for _, service := range toDelete {
		err := r.Delete(context.TODO(), &service)
		if err != nil {
			return false, nil, err
		}
		servicesInSync = false
	}

	for _, specService := range specServices {
		if contains, idx := listContainsByName(specService, clusterServices); contains {
			clusterService := clusterServices[idx]
			if !cmp.Equal(specService, clusterService, serviceDiffOpts) {
				r.Log.Info(fmt.Sprintf("Updating service: %s", clusterService.Name))
				if r.DebugLogging {
					r.Log.Info(fmt.Sprintf("Diff: %s", cmp.Diff(specService, clusterService, serviceDiffOpts)))
				}
				// Cannot naively copy spec, as clusterIP is unmodifiable
				clusterIP := clusterService.Spec.ClusterIP
				clusterService.Spec = specService.Spec
				clusterService.Spec.ClusterIP = clusterIP
				err := r.Update(context.TODO(), &clusterService)
				if err != nil && !errors.IsConflict(err) {
					return false, nil, err
				}
				servicesInSync = false
			}
		} else {
			err := r.Create(context.TODO(), &specService)
			if err != nil {
				return false, nil, err
			}
			servicesInSync = false
		}
	}

	return servicesInSync, clusterServices, nil
}

func (r *DevWorkspaceRoutingReconciler) getClusterServices(routing *controllerv1alpha1.DevWorkspaceRouting) ([]corev1.Service, error) {
	found := &corev1.ServiceList{}
	labelSelector, err := labels.Parse(fmt.Sprintf("%s=%s", constants.DevWorkspaceIDLabel, routing.Spec.DevWorkspaceId))
	if err != nil {
		return nil, err
	}
	listOptions := &client.ListOptions{
		Namespace:     routing.Namespace,
		LabelSelector: labelSelector,
	}
	err = r.List(context.TODO(), found, listOptions)
	if err != nil {
		return nil, err
	}
	return found.Items, nil
}

func getServicesToDelete(clusterServices, specServices []corev1.Service) []corev1.Service {
	var toDelete []corev1.Service
	for _, clusterService := range clusterServices {
		if contains, _ := listContainsByName(clusterService, specServices); !contains {
			toDelete = append(toDelete, clusterService)
		}
	}
	return toDelete
}

func listContainsByName(query corev1.Service, list []corev1.Service) (exists bool, idx int) {
	for idx, listService := range list {
		if query.Name == listService.Name {
			return true, idx
		}
	}
	return false, -1
}
