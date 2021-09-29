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

	"github.com/devfile/devworkspace-operator/pkg/constants"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
)

var ingressDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(networkingv1.Ingress{}, "TypeMeta", "ObjectMeta", "Status"),
	cmpopts.IgnoreFields(networkingv1.HTTPIngressPath{}, "PathType"),
}

func (r *DevWorkspaceRoutingReconciler) syncIngresses(routing *controllerv1alpha1.DevWorkspaceRouting, specIngresses []networkingv1.Ingress) (ok bool, clusterIngresses []networkingv1.Ingress, err error) {
	ingressesInSync := true

	clusterIngresses, err = r.getClusterIngresses(routing)
	if err != nil {
		return false, nil, err
	}

	toDelete := getIngressesToDelete(clusterIngresses, specIngresses)
	for _, ingress := range toDelete {
		err := r.Delete(context.TODO(), &ingress)
		if err != nil {
			return false, nil, err
		}
		ingressesInSync = false
	}

	for _, specIngress := range specIngresses {
		if contains, idx := listContainsIngressByName(specIngress, clusterIngresses); contains {
			clusterIngress := clusterIngresses[idx]
			if !cmp.Equal(specIngress, clusterIngress, ingressDiffOpts) {
				r.Log.Info(fmt.Sprintf("Updating ingress: %s", clusterIngress.Name))
				if r.DebugLogging {
					r.Log.Info(fmt.Sprintf("Diff: %s", cmp.Diff(specIngress, clusterIngress, ingressDiffOpts)))
				}
				// Update ingress's spec
				clusterIngress.Spec = specIngress.Spec
				err := r.Update(context.TODO(), &clusterIngress)
				if err != nil && !errors.IsConflict(err) {
					return false, nil, err
				}
				ingressesInSync = false
			}
		} else {
			err := r.Create(context.TODO(), &specIngress)
			if err != nil {
				return false, nil, err
			}
			ingressesInSync = false
		}
	}

	return ingressesInSync, clusterIngresses, nil
}

func (r *DevWorkspaceRoutingReconciler) getClusterIngresses(routing *controllerv1alpha1.DevWorkspaceRouting) ([]networkingv1.Ingress, error) {
	found := &networkingv1.IngressList{}
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

func getIngressesToDelete(clusterIngresses, specIngresses []networkingv1.Ingress) []networkingv1.Ingress {
	var toDelete []networkingv1.Ingress
	for _, clusterIngress := range clusterIngresses {
		if contains, _ := listContainsIngressByName(clusterIngress, specIngresses); !contains {
			toDelete = append(toDelete, clusterIngress)
		}
	}
	return toDelete
}

func listContainsIngressByName(query networkingv1.Ingress, list []networkingv1.Ingress) (exists bool, idx int) {
	for idx, listIngress := range list {
		if query.Name == listIngress.Name {
			return true, idx
		}
	}
	return false, -1
}
