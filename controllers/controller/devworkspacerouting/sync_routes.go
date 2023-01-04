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
	routeV1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
)

func (r *DevWorkspaceRoutingReconciler) syncRoutes(routing *controllerv1alpha1.DevWorkspaceRouting, specRoutes []routeV1.Route) (ok bool, clusterRoutes []routeV1.Route, err error) {
	routesInSync := true

	clusterRoutes, err = r.getClusterRoutes(routing)
	if err != nil {
		return false, nil, err
	}

	toDelete := getRoutesToDelete(clusterRoutes, specRoutes)
	for _, route := range toDelete {
		err := r.Delete(context.TODO(), &route)
		if err != nil {
			return false, nil, err
		}
		routesInSync = false
	}

	clusterAPI := sync.ClusterAPI{
		Client: r.Client,
		Scheme: r.Scheme,
		Logger: r.Log.WithValues("Request.Namespace", routing.Namespace, "Request.Name", routing.Name),
		Ctx:    context.TODO(),
	}

	var updatedClusterRoutes []routeV1.Route
	for _, specRoute := range specRoutes {
		clusterObj, err := sync.SyncObjectWithCluster(&specRoute, clusterAPI)
		switch t := err.(type) {
		case nil:
			break
		case *sync.NotInSyncError:
			routesInSync = false
			continue
		case *sync.UnrecoverableSyncError:
			return false, nil, t.Cause
		default:
			return false, nil, err
		}
		updatedClusterRoutes = append(updatedClusterRoutes, *clusterObj.(*routeV1.Route))
	}

	return routesInSync, updatedClusterRoutes, nil
}

func (r *DevWorkspaceRoutingReconciler) getClusterRoutes(routing *controllerv1alpha1.DevWorkspaceRouting) ([]routeV1.Route, error) {
	found := &routeV1.RouteList{}
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

	var routes []routeV1.Route
	for _, route := range found.Items {
		for _, ownerref := range route.OwnerReferences {
			// We need to filter routes that are created automatically for ingresses on OpenShift
			if ownerref.Kind == "Ingress" {
				continue
			}
			routes = append(routes, route)
		}
	}
	return routes, nil
}

func getRoutesToDelete(clusterRoutes, specRoutes []routeV1.Route) []routeV1.Route {
	var toDelete []routeV1.Route
	for _, clusterRoute := range clusterRoutes {
		if contains, _ := listContainsRouteByName(clusterRoute, specRoutes); !contains {
			toDelete = append(toDelete, clusterRoute)
		}
	}
	return toDelete
}

func listContainsRouteByName(query routeV1.Route, list []routeV1.Route) (exists bool, idx int) {
	for idx, listRoute := range list {
		if query.Name == listRoute.Name {
			return true, idx
		}
	}
	return false, -1
}
