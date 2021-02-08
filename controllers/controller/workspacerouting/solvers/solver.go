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
	"fmt"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config"
	routeV1 "github.com/openshift/api/route/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RoutingObjects struct {
	Services     []v1.Service
	Ingresses    []v1beta1.Ingress
	Routes       []routeV1.Route
	PodAdditions *controllerv1alpha1.PodAdditions
}

type RoutingSolver interface {
	// FinalizerRequired tells the caller if the solver requires a finalizer on the routing object.
	FinalizerRequired(routing *controllerv1alpha1.WorkspaceRouting) bool

	// Finalize implements the custom finalization logic required by the solver. The solver doesn't have to
	// remove any finalizer from the finalizer list on the routing. Instead just implement the custom
	// logic required for the finalization itself. If this method doesn't return any error, the finalizer
	// is automatically removed from the routing.
	Finalize(routing *controllerv1alpha1.WorkspaceRouting) error

	// GetSpecObjects constructs cluster routing objects which should be applied on the cluster
	// This method should return RoutingNotReady error if the solver is not ready yet to process
	// the workspace routing, RoutingInvalid error if there is a specific reason for the failure or
	// any other error.
	// The implementors can also create any additional objects not captured by the RoutingObjects struct. If that's
	// the case they are required to set the restricted access annotation on any objects created according to the
	// restricted access specified by the routing.
	GetSpecObjects(routing *controllerv1alpha1.WorkspaceRouting, workspaceMeta WorkspaceMetadata) (RoutingObjects, error)

	// GetExposedEndpoints retreives the URL for each endpoint in a devfile spec from a set of RoutingObjects.
	// Returns is a map from component ids (as defined in the devfile) to the list of endpoints for that component
	// Return value "ready" specifies if all endpoints are resolved on the cluster; if false it is necessary to retry, as
	// URLs will be undefined.
	GetExposedEndpoints(endpoints map[string]controllerv1alpha1.EndpointList, routingObj RoutingObjects) (exposedEndpoints map[string]controllerv1alpha1.ExposedEndpointList, ready bool, err error)
}

type RoutingSolverGetter interface {
	// SetupControllerManager is called during the setup of the controller and can modify the controller manager with additional
	// watches, etc., needed for the correct operation of the solver.
	SetupControllerManager(mgr *builder.Builder) error

	// HasSolver returns whether the provided routingClass is supported by this RoutingSolverGetter. Returns false if
	// calling GetSolver with routingClass will return a RoutingNotSupported error. Can be used to check if a routingClass
	// is supported without having to provide a runtime client. Note that GetSolver may still return another error, if e.g.
	// an OpenShift-only routingClass is used on a vanilla Kubernetes platform.
	HasSolver(routingClass controllerv1alpha1.WorkspaceRoutingClass) bool

	// GetSolver that obtains a Solver (see github.com/devfile/devworkspace-operator/controllers/controller/workspacerouting/solvers)
	// for a particular WorkspaceRouting instance. This function should return a RoutingNotSupported error if
	// the routingClass is not recognized, and any other error if the routingClass is invalid (e.g. an OpenShift-only
	// routingClass on a vanilla Kubernetes platform). Note that an empty routingClass is handled by the DevWorkspace controller itself,
	// and should not be handled by external controllers.
	GetSolver(client client.Client, routingClass controllerv1alpha1.WorkspaceRoutingClass) (solver RoutingSolver, err error)
}

type SolverGetter struct{}

var _ RoutingSolverGetter = (*SolverGetter)(nil)

func (_ *SolverGetter) HasSolver(routingClass controllerv1alpha1.WorkspaceRoutingClass) bool {
	if routingClass == "" {
		// Special case for built-in: empty routing class returns the default solver for the DevWorkspace controller.
		return true
	}
	switch routingClass {
	case controllerv1alpha1.WorkspaceRoutingBasic,
		controllerv1alpha1.WorkspaceRoutingOpenShiftOauth,
		controllerv1alpha1.WorkspaceRoutingCluster,
		controllerv1alpha1.WorkspaceRoutingClusterTLS,
		controllerv1alpha1.WorkspaceRoutingWebTerminal:
		return true
	default:
		return false
	}
}

func (_ *SolverGetter) GetSolver(client client.Client, routingClass controllerv1alpha1.WorkspaceRoutingClass) (RoutingSolver, error) {
	if routingClass == "" {
		routingClass = controllerv1alpha1.WorkspaceRoutingClass(config.ControllerCfg.GetDefaultRoutingClass())
	}
	switch routingClass {
	case controllerv1alpha1.WorkspaceRoutingBasic:
		return &BasicSolver{}, nil
	case controllerv1alpha1.WorkspaceRoutingOpenShiftOauth:
		if !config.ControllerCfg.IsOpenShift() {
			return nil, fmt.Errorf("routing class %s only supported on OpenShift", routingClass)
		}
		return &OpenShiftOAuthSolver{Client: client}, nil
	case controllerv1alpha1.WorkspaceRoutingCluster:
		return &ClusterSolver{}, nil
	case controllerv1alpha1.WorkspaceRoutingClusterTLS, controllerv1alpha1.WorkspaceRoutingWebTerminal:
		if !config.ControllerCfg.IsOpenShift() {
			return nil, fmt.Errorf("routing class %s only supported on OpenShift", routingClass)
		}
		return &ClusterSolver{TLS: true}, nil
	default:
		return nil, RoutingNotSupported
	}
}

func (*SolverGetter) SetupControllerManager(mgr *builder.Builder) error {
	return nil
}
