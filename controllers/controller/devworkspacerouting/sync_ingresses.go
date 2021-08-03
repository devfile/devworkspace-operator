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
