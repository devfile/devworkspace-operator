//
// Copyright (c) 2019-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package workspacerouting

import (
	"context"
	"fmt"

	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
)

var ingressDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(v1beta1.Ingress{}, "TypeMeta", "ObjectMeta", "Status"),
	cmpopts.IgnoreFields(v1beta1.HTTPIngressPath{}, "PathType"),
}

func (r *WorkspaceRoutingReconciler) syncIngresses(routing *controllerv1alpha1.WorkspaceRouting, specIngresses []v1beta1.Ingress) (ok bool, clusterIngresses []v1beta1.Ingress, err error) {
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

func (r *WorkspaceRoutingReconciler) getClusterIngresses(routing *controllerv1alpha1.WorkspaceRouting) ([]v1beta1.Ingress, error) {
	found := &v1beta1.IngressList{}
	labelSelector, err := labels.Parse(fmt.Sprintf("%s=%s", config.WorkspaceIDLabel, routing.Spec.WorkspaceId))
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

func getIngressesToDelete(clusterIngresses, specIngresses []v1beta1.Ingress) []v1beta1.Ingress {
	var toDelete []v1beta1.Ingress
	for _, clusterIngress := range clusterIngresses {
		if contains, _ := listContainsIngressByName(clusterIngress, specIngresses); !contains {
			toDelete = append(toDelete, clusterIngress)
		}
	}
	return toDelete
}

func listContainsIngressByName(query v1beta1.Ingress, list []v1beta1.Ingress) (exists bool, idx int) {
	for idx, listIngress := range list {
		if query.Name == listIngress.Name {
			return true, idx
		}
	}
	return false, -1
}
