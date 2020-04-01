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

package solvers

import (
	"github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
)

var ingressAnnotations = map[string]string{
	"kubernetes.io/ingress.class":                "nginx",
	"nginx.ingress.kubernetes.io/rewrite-target": "/",
	"nginx.ingress.kubernetes.io/ssl-redirect":   "false",
}

type BasicSolver struct{}

var _ RoutingSolver = (*BasicSolver)(nil)

func (s *BasicSolver) GetSpecObjects(spec v1alpha1.WorkspaceRoutingSpec, workspaceMeta WorkspaceMetadata) RoutingObjects {
	services := getServicesForEndpoints(spec.Endpoints, workspaceMeta)
	services = append(services, getDiscoverableServicesForEndpoints(spec.Endpoints, workspaceMeta)...)
	ingresses, exposedEndpoints := getIngressesForSpec(spec.Endpoints, workspaceMeta)

	return RoutingObjects{
		Services:         services,
		Ingresses:        ingresses,
		ExposedEndpoints: exposedEndpoints,
	}
}
