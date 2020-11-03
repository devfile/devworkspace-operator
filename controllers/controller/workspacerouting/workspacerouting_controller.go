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

	"github.com/devfile/devworkspace-operator/controllers/controller/workspacerouting/solvers"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/google/go-cmp/cmp"
	routeV1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
)

const workspaceRoutingFinalizer = "workspacerouting.controller.devfile.io"

// WorkspaceRoutingReconciler reconciles a WorkspaceRouting object
type WorkspaceRoutingReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=controller.devfile.io,resources=workspaceroutings,verbs=*
// +kubebuilder:rbac:groups=controller.devfile.io,resources=workspaceroutings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=*
// +kubebuilder:rbac:groups=extensions,resources=ingresses,verbs=*
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=*
// +kubebuidler:rbac:groups=route.openshift.io,resources=routes/status,verbs=get,list,watch
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes/custom-host,verbs=create

func (r *WorkspaceRoutingReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling WorkspaceRouting")

	// Fetch the WorkspaceRouting instance
	instance := &controllerv1alpha1.WorkspaceRouting{}
	err := r.Get(ctx, req.NamespacedName, instance)
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
	if instance.GetDeletionTimestamp() != nil {
		reqLogger.Info("Finalizing WorkspaceRouting")
		return reconcile.Result{}, r.finalize(instance)
	}

	if instance.Status.Phase == controllerv1alpha1.RoutingFailed {
		return reconcile.Result{}, err
	}

	solver, err := getSolverForRoutingClass(instance.Spec.RoutingClass)
	if err != nil {
		reqLogger.Error(err, "Could not get solver for routingClass")
		instance.Status.Phase = controllerv1alpha1.RoutingFailed
		statusErr := r.Status().Update(ctx, instance)
		return reconcile.Result{}, statusErr
	}

	// Add finalizer for this CR if not already present
	if err := r.setFinalizer(reqLogger, instance); err != nil {
		return reconcile.Result{}, err
	}

	workspaceMeta := solvers.WorkspaceMetadata{
		WorkspaceId:   instance.Spec.WorkspaceId,
		Namespace:     instance.Namespace,
		PodSelector:   instance.Spec.PodSelector,
		RoutingSuffix: instance.Spec.RoutingSuffix,
	}

	routingObjects := solver.GetSpecObjects(instance.Spec, workspaceMeta)
	services := routingObjects.Services
	for idx := range services {
		err := controllerutil.SetControllerReference(instance, &services[idx], r.Scheme)
		if err != nil {
			return reconcile.Result{}, err
		}
	}
	ingresses := routingObjects.Ingresses
	for idx := range ingresses {
		err := controllerutil.SetControllerReference(instance, &ingresses[idx], r.Scheme)
		if err != nil {
			return reconcile.Result{}, err
		}
	}
	routes := routingObjects.Routes
	for idx := range routes {
		err := controllerutil.SetControllerReference(instance, &routes[idx], r.Scheme)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	servicesInSync, clusterServices, err := r.syncServices(instance, services)
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

	clusterRoutingObj := solvers.RoutingObjects{
		Services:  clusterServices,
		Ingresses: clusterIngresses,
		Routes:    clusterRoutes,
	}
	exposedEndpoints, endpointsAreReady, err := solver.GetExposedEndpoints(instance.Spec.Endpoints, clusterRoutingObj)
	if err != nil {
		reqLogger.Error(err, "Could not get exposed endpoints for workspace")
		instance.Status.Phase = controllerv1alpha1.RoutingFailed
		statusErr := r.Status().Update(ctx, instance)
		return reconcile.Result{}, statusErr
	}

	if config.ControllerCfg.IsOpenShift() {
		oauthClient := routingObjects.OAuthClient
		oauthClientInSync, err := r.syncOAuthClient(instance, oauthClient)
		if err != nil || !oauthClientInSync {
			reqLogger.Info("OAuthClient not in sync")
			return reconcile.Result{Requeue: true}, err
		}
	}

	return reconcile.Result{}, r.reconcileStatus(instance, routingObjects, exposedEndpoints, endpointsAreReady)
}

// setFinalizer ensures a finalizer is set on a workspaceRouting instance; no-op if finalizer is already present.
func (r *WorkspaceRoutingReconciler) setFinalizer(reqLogger logr.Logger, m *controllerv1alpha1.WorkspaceRouting) error {
	if !isFinalizerNecessary(m) || contains(m.GetFinalizers(), workspaceRoutingFinalizer) {
		return nil
	}
	reqLogger.Info("Adding Finalizer for the WorkspaceRouting")
	m.SetFinalizers(append(m.GetFinalizers(), workspaceRoutingFinalizer))

	// Update CR
	err := r.Update(context.TODO(), m)
	if err != nil {
		reqLogger.Error(err, "Failed to update WorkspaceRouting with finalizer")
		return err
	}
	return nil
}

func (r *WorkspaceRoutingReconciler) finalize(instance *controllerv1alpha1.WorkspaceRouting) error {
	if contains(instance.GetFinalizers(), workspaceRoutingFinalizer) {
		// Run finalization logic for workspaceRoutingFinalizer. If the
		// finalization logic fails, don't remove the finalizer so
		// that we can retry during the next reconciliation.
		if err := r.deleteOAuthClients(instance); err != nil {
			return err
		}
		// Remove workspaceRoutingFinalizer. Once all finalizers have been
		// removed, the object will be deleted.
		instance.SetFinalizers(remove(instance.GetFinalizers(), workspaceRoutingFinalizer))
		err := r.Update(context.TODO(), instance)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *WorkspaceRoutingReconciler) reconcileStatus(
	instance *controllerv1alpha1.WorkspaceRouting,
	routingObjects solvers.RoutingObjects,
	exposedEndpoints map[string]controllerv1alpha1.ExposedEndpointList,
	endpointsReady bool) error {

	if !endpointsReady {
		instance.Status.Phase = controllerv1alpha1.RoutingPreparing
		return r.Status().Update(context.TODO(), instance)
	}
	if instance.Status.Phase == controllerv1alpha1.RoutingReady &&
		cmp.Equal(instance.Status.PodAdditions, routingObjects.PodAdditions) &&
		cmp.Equal(instance.Status.ExposedEndpoints, exposedEndpoints) {
		return nil
	}
	instance.Status.Phase = controllerv1alpha1.RoutingReady
	instance.Status.PodAdditions = routingObjects.PodAdditions
	instance.Status.ExposedEndpoints = exposedEndpoints
	return r.Status().Update(context.TODO(), instance)
}

func getSolverForRoutingClass(routingClass controllerv1alpha1.WorkspaceRoutingClass) (solvers.RoutingSolver, error) {
	if routingClass == "" {
		routingClass = controllerv1alpha1.WorkspaceRoutingClass(config.ControllerCfg.GetDefaultRoutingClass())
	}
	switch routingClass {
	case controllerv1alpha1.WorkspaceRoutingDefault:
		return &solvers.BasicSolver{}, nil
	case controllerv1alpha1.WorkspaceRoutingOpenShiftOauth:
		if !config.ControllerCfg.IsOpenShift() {
			return nil, fmt.Errorf("routing class %s only supported on OpenShift", routingClass)
		}
		return &solvers.OpenShiftOAuthSolver{}, nil
	case controllerv1alpha1.WorkspaceRoutingCluster:
		return &solvers.ClusterSolver{}, nil
	case controllerv1alpha1.WorkspaceRoutingClusterTLS, controllerv1alpha1.WorkspaceRoutingWebTerminal:
		if !config.ControllerCfg.IsOpenShift() {
			return nil, fmt.Errorf("routing class %s only supported on OpenShift", routingClass)
		}
		return &solvers.ClusterSolver{TLS: true}, nil
	default:
		return nil, fmt.Errorf("routing class %s not supported", routingClass)
	}
}

func isFinalizerNecessary(routing *controllerv1alpha1.WorkspaceRouting) bool {
	routingClass := routing.Spec.RoutingClass
	if routingClass == "" {
		routingClass = controllerv1alpha1.WorkspaceRoutingClass(config.ControllerCfg.GetDefaultRoutingClass())
	}
	switch routingClass {
	case controllerv1alpha1.WorkspaceRoutingOpenShiftOauth:
		return true
	case controllerv1alpha1.WorkspaceRoutingDefault:
		return false
	default:
		return false
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

func (r *WorkspaceRoutingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&controllerv1alpha1.WorkspaceRouting{}).
		Owns(&corev1.Service{}).
		Owns(&v1beta1.Ingress{})
	if config.ControllerCfg.IsOpenShift() {
		builder.Owns(&routeV1.Route{})
	}
	return builder.Complete(r)
}
