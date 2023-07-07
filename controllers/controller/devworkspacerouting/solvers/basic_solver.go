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

package solvers

import (
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
)

var routeAnnotations = func(endpointName string) map[string]string {
	return map[string]string{
		"haproxy.router.openshift.io/rewrite-target": "/",
		constants.DevWorkspaceEndpointNameAnnotation: endpointName,
	}
}

var nginxIngressAnnotations = func(endpointName string) map[string]string {
	return map[string]string{
		"nginx.ingress.kubernetes.io/rewrite-target": "/",
		"nginx.ingress.kubernetes.io/ssl-redirect":   "false",
		constants.DevWorkspaceEndpointNameAnnotation: endpointName,
	}
}

// Basic solver exposes endpoints without any authentication
// According to the current cluster there is different behavior:
// Kubernetes: use Ingresses without TLS
// OpenShift: use Routes with TLS enabled
type BasicSolver struct{}

var _ RoutingSolver = (*BasicSolver)(nil)

func (s *BasicSolver) FinalizerRequired(*controllerv1alpha1.DevWorkspaceRouting) bool {
	return false
}

func (s *BasicSolver) Finalize(*controllerv1alpha1.DevWorkspaceRouting) error {
	return nil
}

func (s *BasicSolver) GetSpecObjects(routing *controllerv1alpha1.DevWorkspaceRouting, workspaceMeta DevWorkspaceMetadata) (RoutingObjects, error) {
	routingObjects := RoutingObjects{}

	// TODO: Use workspace-scoped ClusterHostSuffix to allow overriding
	routingSuffix := config.GetGlobalConfig().Routing.ClusterHostSuffix
	if routingSuffix == "" {
		return routingObjects, &RoutingInvalid{"basic routing requires .config.routing.clusterHostSuffix to be set in operator config"}
	}

	spec := routing.Spec
	services := getServicesForEndpoints(spec.Endpoints, workspaceMeta)
	services = append(services, GetDiscoverableServicesForEndpoints(spec.Endpoints, workspaceMeta)...)
	routingObjects.Services = services
	if infrastructure.IsOpenShift() {
		routingObjects.Routes = getRoutesForSpec(routingSuffix, spec.Endpoints, workspaceMeta)
	} else {
		routingObjects.Ingresses = getIngressesForSpec(routingSuffix, spec.Endpoints, workspaceMeta)
	}

	return routingObjects, nil
}

func (s *BasicSolver) GetExposedEndpoints(
	endpoints map[string]controllerv1alpha1.EndpointList,
	routingObj RoutingObjects) (exposedEndpoints map[string]controllerv1alpha1.ExposedEndpointList, ready bool, err error) {
	return getExposedEndpoints(endpoints, routingObj)
}
