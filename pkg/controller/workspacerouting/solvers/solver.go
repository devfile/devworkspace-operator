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
	v12 "github.com/openshift/api/route/v1"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
)

type RoutingObjects struct {
	Services         []v1.Service
	Ingresses        []v1beta1.Ingress
	Routes           []v12.Route
	PodAdditions     *v1alpha1.PodAdditions
	ExposedEndpoints map[string][]v1alpha1.ExposedEndpoint
}

type RoutingSolver interface {
	GetSpecObjects(spec v1alpha1.WorkspaceRoutingSpec, workspaceMeta WorkspaceMetadata) RoutingObjects
}
