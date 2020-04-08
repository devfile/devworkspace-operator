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
	"github.com/go-logr/logr"
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

const workspaceRoutingFinalizer = "workspacerouting.workspace.eclipse.org"

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

	// Check if the WorkspaceRouting instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isRoutingMarkedToBeDeleted := instance.GetDeletionTimestamp() != nil
	if isRoutingMarkedToBeDeleted {
		if contains(instance.GetFinalizers(), workspaceRoutingFinalizer) {
			// Run finalization logic for workspaceRoutingFinalizer. If the
			// finalization logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.deleteOAuthClients(instance); err != nil {
				return reconcile.Result{}, err
			}
			// Remove workspaceRoutingFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			instance.SetFinalizers(remove(instance.GetFinalizers(), workspaceRoutingFinalizer))
			err := r.client.Update(context.TODO(), instance)
			if err != nil {
				return reconcile.Result{}, err
			}
			reqLogger.Info("WorkspaceRouting is finalized")
		}
		return reconcile.Result{}, nil
	}

	// Add finalizer for this CR
	if !contains(instance.GetFinalizers(), workspaceRoutingFinalizer) {
		if err := r.addFinalizer(reqLogger, instance); err != nil {
			return reconcile.Result{}, err
		}
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

	oauthClient := routingObjects.OAuthClient

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

	exposedEndpoints, endpointsAreReady, err := getExposedEndpoints(instance.Spec.Endpoints, clusterIngresses, clusterRoutes)
	if err != nil {
		reqLogger.Error(err, "Could not get exposed endpoints for workspace")
		instance.Status.Phase = workspacev1alpha1.RoutingFailed
		statusErr := r.client.Status().Update(context.TODO(), instance)
		return reconcile.Result{}, statusErr
	}

	oauthClientInSync, err := r.syncOAuthClient(instance, *oauthClient)
	if err != nil || !oauthClientInSync {
		reqLogger.Info("OAuthClient not in sync")
		return reconcile.Result{Requeue: true}, err
	}

	return reconcile.Result{}, r.reconcileStatus(instance, routingObjects, exposedEndpoints, endpointsAreReady)
}

func (r *ReconcileWorkspaceRouting) addFinalizer(reqLogger logr.Logger, m *workspacev1alpha1.WorkspaceRouting) error {
	reqLogger.Info("Adding Finalizer for the WorkspaceRouting")
	m.SetFinalizers(append(m.GetFinalizers(), workspaceRoutingFinalizer))

	// Update CR
	err := r.client.Update(context.TODO(), m)
	if err != nil {
		reqLogger.Error(err, "Failed to update WorkspaceRouting with finalizer")
		return err
	}
	return nil
}

func (r *ReconcileWorkspaceRouting) reconcileStatus(
	instance *workspacev1alpha1.WorkspaceRouting,
	routingObjects solvers.RoutingObjects,
	exposedEndpoints map[string][]workspacev1alpha1.ExposedEndpoint,
	endpointsReady bool) error {

	if !endpointsReady {
		instance.Status.Phase = workspacev1alpha1.RoutingPreparing
		return r.client.Status().Update(context.TODO(), instance)
	}
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

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func remove(list []string, s string) []string {
	for i, v := range list {
		if v == s {
			list = append(list[:i], list[i+1:]...)
		}
	}
	return list
}
