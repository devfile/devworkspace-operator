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
	"github.com/che-incubator/che-workspace-operator/internal/cluster"
	workspacev1alpha1 "github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	"github.com/che-incubator/che-workspace-operator/pkg/config"
	"github.com/che-incubator/che-workspace-operator/pkg/controller/workspacerouting/solvers"
	"github.com/google/go-cmp/cmp"
	routeV1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_workspacerouting")

// Add creates a new WorkspaceRouting Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileWorkspaceRouting{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("workspacerouting-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource WorkspaceRouting
	err = c.Watch(&source.Kind{Type: &workspacev1alpha1.WorkspaceRouting{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resources: Services, Ingresses, and (on OpenShift) Routes.
	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &workspacev1alpha1.WorkspaceRouting{},
	})
	if err != nil {
		return err
	}
	err = c.Watch(&source.Kind{Type: &v1beta1.Ingress{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &workspacev1alpha1.WorkspaceRouting{},
	})
	if err != nil {
		return err
	}

	isOpenShift, err := cluster.IsOpenShift()
	if err != nil {
		log.Error(err, "Failed to determine if running in OpenShift")
		return err
	}
	if isOpenShift {
		err = c.Watch(&source.Kind{Type: &routeV1.Route{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &workspacev1alpha1.WorkspaceRouting{},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// blank assignment to verify that ReconcileWorkspaceRouting implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileWorkspaceRouting{}

// ReconcileWorkspaceRouting reconciles a WorkspaceRouting object
type ReconcileWorkspaceRouting struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a WorkspaceRouting object and makes changes based on the state read
// and what is in the WorkspaceRouting.Spec
func (r *ReconcileWorkspaceRouting) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling WorkspaceRouting")

	// Fetch the WorkspaceRouting instance
	instance := &workspacev1alpha1.WorkspaceRouting{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	workspaceMeta := solvers.WorkspaceMetadata{
		WorkspaceId:   instance.Spec.WorkspaceId,
		Namespace:     instance.Namespace,
		PodSelector:   instance.Spec.PodSelector,
		RoutingSuffix: instance.Spec.RoutingSuffix,
	}

	if instance.Status.Phase == workspacev1alpha1.RoutingFailed {
		return reconcile.Result{}, err
	}

	solver, err := getSolverForRoutingClass(instance.Spec.RoutingClass)
	if err != nil {
		reqLogger.Error(err, "Could not get solver for routingClass")
		instance.Status.Phase = workspacev1alpha1.RoutingFailed
		statusErr := r.client.Status().Update(context.TODO(), instance)
		return reconcile.Result{}, statusErr
	}

	routingObjects := solver.GetSpecObjects(instance.Spec, workspaceMeta)
	services := routingObjects.Services
	for idx := range services {
		err := controllerutil.SetControllerReference(instance, &services[idx], r.scheme)
		if err != nil {
			return reconcile.Result{}, err
		}
	}
	ingresses := routingObjects.Ingresses
	for idx := range ingresses {
		err := controllerutil.SetControllerReference(instance, &ingresses[idx], r.scheme)
		if err != nil {
			return reconcile.Result{}, err
		}
	}
	routes := routingObjects.Routes
	for idx := range routes {
		err := controllerutil.SetControllerReference(instance, &routes[idx], r.scheme)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	servicesInSync, err := r.syncServices(instance, services)
	if err != nil || !servicesInSync {
		reqLogger.Info("Services not in sync")
		return reconcile.Result{Requeue: true}, err
	}

	ingressesInSync, clusterIngresses, err := r.syncIngresses(instance, ingresses)
	if err != nil || !ingressesInSync {
		reqLogger.Info("Ingresses not in sync")
		return reconcile.Result{Requeue: true}, err
	}

	routesInSync, clusterRoutes, err := r.syncRoutes(instance, routes)
	if err != nil || !routesInSync {
		reqLogger.Info("Routes not in sync")
		return reconcile.Result{Requeue: true}, err
	}

	exposedEndpoints, err := getExposedEndpoints(instance.Spec.Endpoints, clusterIngresses, clusterRoutes)
	if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, r.reconcileStatus(instance, routingObjects, exposedEndpoints)
}

func (r *ReconcileWorkspaceRouting) reconcileStatus(
		instance *workspacev1alpha1.WorkspaceRouting,
		routingObjects solvers.RoutingObjects,
		exposedEndpoints map[string][]workspacev1alpha1.ExposedEndpoint) error {
	if instance.Status.Phase == workspacev1alpha1.RoutingReady &&
			cmp.Equal(instance.Status.PodAdditions, routingObjects.PodAdditions) &&
			cmp.Equal(instance.Status.ExposedEndpoints, exposedEndpoints) {
		return nil
	}
	instance.Status.Phase = workspacev1alpha1.RoutingReady
	instance.Status.PodAdditions = routingObjects.PodAdditions
	instance.Status.ExposedEndpoints = exposedEndpoints
	return r.client.Status().Update(context.TODO(), instance)
}

func getExposedEndpoints(
		endpoints map[string][]workspacev1alpha1.Endpoint,
		ingresses []v1beta1.Ingress,
		routes []routeV1.Route) (map[string][]workspacev1alpha1.ExposedEndpoint, error) {
	exposedEndpoints := map[string][]workspacev1alpha1.ExposedEndpoint{}

	for machineName, machineEndpoints := range endpoints {
		for _, endpoint := range machineEndpoints {
			if endpoint.Attributes[workspacev1alpha1.PUBLIC_ENDPOINT_ATTRIBUTE] != "true" {
				continue
			}
			url, err := getURLforEndpoint(endpoint, ingresses, routes)
			if err != nil {
				return nil, err
			}
			exposedEndpoints[machineName] = append(exposedEndpoints[machineName], workspacev1alpha1.ExposedEndpoint{
				Name:       endpoint.Name,
				Url:        url,
				Attributes: endpoint.Attributes,
			})
		}
	}
	return exposedEndpoints, nil
}

func getURLforEndpoint(
		endpoint workspacev1alpha1.Endpoint,
		ingresses []v1beta1.Ingress,
		routes []routeV1.Route) (string, error) {
	for _, route := range routes {
		if route.Annotations[config.WorkspaceEndpointNameAnnotation] == endpoint.Name {
			protocol := endpoint.Attributes[workspacev1alpha1.PROTOCOL_ENDPOINT_ATTRIBUTE]
			if endpoint.Attributes[workspacev1alpha1.SECURE_ENDPOINT_ATTRIBUTE] == "true" &&
					route.Spec.TLS != nil {
				protocol = getSecureProtocol(protocol)
			}
			url := fmt.Sprintf("%s://%s", protocol, route.Spec.Host)
			return url, nil
		}
	}
	for _, ingress := range ingresses {
		if ingress.Annotations[config.WorkspaceEndpointNameAnnotation] == endpoint.Name {
			if len(ingress.Spec.Rules) == 1 {
				protocol := endpoint.Attributes[workspacev1alpha1.PROTOCOL_ENDPOINT_ATTRIBUTE]
				url := fmt.Sprintf("%s://%s", protocol, ingress.Spec.Rules[0].Host)
				return url, nil
			} else {
				return "", fmt.Errorf("ingress %s contains multiple rules", ingress.Name)
			}
		}
	}
	return "", fmt.Errorf("could not get URL for endpoint %s", endpoint.Name)
}

func getSolverForRoutingClass(routingClass workspacev1alpha1.WorkspaceRoutingClass) (solvers.RoutingSolver, error) {
	if routingClass == "" {
		routingClass = workspacev1alpha1.WorkspaceRoutingClass(config.ControllerCfg.GetDefaultRoutingClass())
	}
	switch routingClass {
	case workspacev1alpha1.WorkspaceRoutingDefault:
		return &solvers.BasicSolver{}, nil
	case workspacev1alpha1.WorkspaceRoutingOpenShiftOauth:
		return &solvers.OpenShiftOAuthSolver{}, nil
	default:
		return nil, fmt.Errorf("routing class %s not supported", routingClass)
	}
}

// getSecureProtocol takes a (potentially unsecure protocol e.g. http) and returns the secure version (e.g. https).
// If protocol isn't recognized, it is returned unmodified.
func getSecureProtocol(protocol string) string {
	switch protocol {
	case "ws":
		return "wss"
	case "http":
		return "https"
	default:
		return protocol
	}
}

