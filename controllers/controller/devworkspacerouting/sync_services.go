//
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
//

package devworkspacerouting

import (
	"context"
	"fmt"

	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
)

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

	clusterAPI := sync.ClusterAPI{
		Client: r.Client,
		Scheme: r.Scheme,
		Logger: r.Log.WithValues("Request.Namespace", routing.Namespace, "Request.Name", routing.Name),
		Ctx:    context.TODO(),
	}

	var updatedClusterServices []corev1.Service
	for _, specService := range specServices {
		clusterObj, err := sync.SyncObjectWithCluster(&specService, clusterAPI)
		switch t := err.(type) {
		case nil:
			break
		case *sync.NotInSyncError:
			servicesInSync = false
			continue
		case *sync.UnrecoverableSyncError:
			return false, nil, t.Cause
		default:
			return false, nil, err
		}
		updatedClusterServices = append(updatedClusterServices, *clusterObj.(*corev1.Service))
	}

	return servicesInSync, updatedClusterServices, nil
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
