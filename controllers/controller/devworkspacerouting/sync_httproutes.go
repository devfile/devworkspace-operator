//
// Copyright (c) 2019-2026 Red Hat, Inc.
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

package devworkspacerouting

import (
	"context"
	"fmt"

	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"

	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
)

func (r *DevWorkspaceRoutingReconciler) syncHTTPRoutes(routing *controllerv1alpha1.DevWorkspaceRouting, specHTTPRoutes []gwapiv1.HTTPRoute) (ok bool, clusterHTTPRoutes []gwapiv1.HTTPRoute, err error) {
	httpRoutesInSync := true

	clusterHTTPRoutes, err = r.getClusterHTTPRoutes(routing)
	if err != nil {
		return false, nil, err
	}

	toDelete := getHTTPRoutesToDelete(clusterHTTPRoutes, specHTTPRoutes)
	for _, httpRoute := range toDelete {
		err := r.Delete(context.TODO(), &httpRoute)
		if err != nil {
			return false, nil, err
		}
		httpRoutesInSync = false
	}

	clusterAPI := sync.ClusterAPI{
		Client: r.Client,
		Scheme: r.Scheme,
		Logger: r.Log.WithValues("Request.Namespace", routing.Namespace, "Request.Name", routing.Name),
		Ctx:    context.TODO(),
	}

	var updatedClusterHTTPRoutes []gwapiv1.HTTPRoute
	for _, specHTTPRoute := range specHTTPRoutes {
		clusterObj, err := sync.SyncObjectWithCluster(&specHTTPRoute, clusterAPI)
		switch t := err.(type) {
		case nil:
			break
		case *sync.NotInSyncError:
			httpRoutesInSync = false
			continue
		case *sync.UnrecoverableSyncError:
			return false, nil, t
		default:
			return false, nil, err
		}
		updatedClusterHTTPRoutes = append(updatedClusterHTTPRoutes, *clusterObj.(*gwapiv1.HTTPRoute))
	}

	return httpRoutesInSync, updatedClusterHTTPRoutes, nil
}

func (r *DevWorkspaceRoutingReconciler) getClusterHTTPRoutes(routing *controllerv1alpha1.DevWorkspaceRouting) ([]gwapiv1.HTTPRoute, error) {
	found := &gwapiv1.HTTPRouteList{}
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

func getHTTPRoutesToDelete(clusterHTTPRoutes, specHTTPRoutes []gwapiv1.HTTPRoute) []gwapiv1.HTTPRoute {
	var toDelete []gwapiv1.HTTPRoute
	for _, clusterHTTPRoute := range clusterHTTPRoutes {
		if contains, _ := listContainsHTTPRouteByName(clusterHTTPRoute, specHTTPRoutes); !contains {
			toDelete = append(toDelete, clusterHTTPRoute)
		}
	}
	return toDelete
}

func listContainsHTTPRouteByName(query gwapiv1.HTTPRoute, list []gwapiv1.HTTPRoute) (exists bool, idx int) {
	for idx, listHTTPRoute := range list {
		if query.Name == listHTTPRoute.Name {
			return true, idx
		}
	}
	return false, -1
}
