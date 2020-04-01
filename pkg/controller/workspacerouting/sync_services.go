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
	"github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	"github.com/che-incubator/che-workspace-operator/pkg/config"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

var serviceDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(corev1.Service{}, "TypeMeta", "ObjectMeta", "Status"),
	cmpopts.IgnoreFields(corev1.ServiceSpec{}, "ClusterIP", "SessionAffinity"),
	cmpopts.IgnoreFields(corev1.ServicePort{}, "TargetPort"),
	cmpopts.SortSlices(func(a, b corev1.ServicePort) bool {
		return strings.Compare(a.Name, b.Name) > 0
	}),
}

func (r *ReconcileWorkspaceRouting) syncServices(routing *v1alpha1.WorkspaceRouting, specServices []corev1.Service) (ok bool, err error) {
	servicesInSync := true

	clusterServices, err := r.getClusterServices(routing)
	if err != nil {
		return false, err
	}

	toDelete := getServicesToDelete(clusterServices, specServices)
	for _, service := range toDelete {
		err := r.client.Delete(context.TODO(), &service)
		if err != nil {
			return false, err
		}
		servicesInSync = false
	}

	for _, specService := range specServices {
		if contains, idx := listContainsByName(specService, clusterServices); contains {
			clusterService := clusterServices[idx]
			if !cmp.Equal(specService, clusterService, serviceDiffOpts) {
				// TODO: jsonpatch appears to ignore array fields, meaning that this is a no-op if ServicePorts are changed.
				// We also cannot use client.Update, as the spec contains ClusterIP which cannot be modified.
				patch := client.MergeFrom(&specService)
				err := r.client.Patch(context.TODO(), &clusterService, patch)
				if err != nil && !errors.IsConflict(err) {
					return false, err
				}
				servicesInSync = false
			}
		} else {
			err := r.client.Create(context.TODO(), &specService)
			if err != nil {
				return false, err
			}
			servicesInSync = false
		}
	}

	return servicesInSync, nil
}

func (r *ReconcileWorkspaceRouting) getClusterServices(routing *v1alpha1.WorkspaceRouting) ([]corev1.Service, error) {
	found := &corev1.ServiceList{}
	labelSelector, err := labels.Parse(fmt.Sprintf("%s=%s", config.WorkspaceIDLabel, routing.Spec.WorkspaceId))
	if err != nil {
		return nil, err
	}
	listOptions := &client.ListOptions{
		Namespace:     routing.Namespace,
		LabelSelector: labelSelector,
	}
	err = r.client.List(context.TODO(), found, listOptions)
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
