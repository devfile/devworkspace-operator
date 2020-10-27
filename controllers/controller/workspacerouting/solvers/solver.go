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
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	routeV1 "github.com/openshift/api/route/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
)

type RoutingObjects struct {
	Services     []v1.Service
	Ingresses    []v1beta1.Ingress
	Routes       []routeV1.Route
	PodAdditions *controllerv1alpha1.PodAdditions
	OAuthClient  *oauthv1.OAuthClient
}

type RoutingSolver interface {
	GetSpecObjects(spec controllerv1alpha1.WorkspaceRoutingSpec, workspaceMeta WorkspaceMetadata) RoutingObjects
	GetExposedEndpoints(endpoints map[string]controllerv1alpha1.EndpointList, routingObj RoutingObjects) (exposedEndpoints map[string]controllerv1alpha1.ExposedEndpointList, ready bool, err error)
}
