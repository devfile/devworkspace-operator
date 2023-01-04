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

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
)

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

	clusterAPI := sync.ClusterAPI{
		Client: r.Client,
		Scheme: r.Scheme,
		Logger: r.Log.WithValues("Request.Namespace", routing.Namespace, "Request.Name", routing.Name),
		Ctx:    context.TODO(),
	}

	var updatedClusterIngresses []networkingv1.Ingress
	for _, specIngress := range specIngresses {
		clusterObj, err := sync.SyncObjectWithCluster(&specIngress, clusterAPI)
		switch t := err.(type) {
		case nil:
			break
		case *sync.NotInSyncError:
			ingressesInSync = false
			continue
		case *sync.UnrecoverableSyncError:
			return false, nil, t.Cause
		default:
			return false, nil, err
		}
		updatedClusterIngresses = append(updatedClusterIngresses, *clusterObj.(*networkingv1.Ingress))
	}

	return ingressesInSync, updatedClusterIngresses, nil
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
