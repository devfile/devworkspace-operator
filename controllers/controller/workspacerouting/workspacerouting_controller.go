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
	"errors"
	"time"

	"github.com/devfile/devworkspace-operator/controllers/controller/workspacerouting/solvers"
	maputils "github.com/devfile/devworkspace-operator/internal/map"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/google/go-cmp/cmp"

	"github.com/go-logr/logr"
	routeV1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
)

var (
	NoSolversEnabled = errors.New("reconciler does not define SolverGetter")
)

const workspaceRoutingFinalizer = "workspacerouting.controller.devfile.io"

// WorkspaceRoutingReconciler reconciles a WorkspaceRouting object
type WorkspaceRoutingReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	// SolverGetter will be used to get solvers for a particular workspaceRouting
	SolverGetter solvers.RoutingSolverGetter
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

	// Fetch the WorkspaceRouting instance
	instance := &controllerv1alpha1.WorkspaceRouting{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	reqLogger = reqLogger.WithValues(config.WorkspaceIDLoggerKey, instance.Spec.WorkspaceId)
	reqLogger.Info("Reconciling WorkspaceRouting")

	solver, err := r.SolverGetter.GetSolver(r.Client, instance.Spec.RoutingClass)
	if err != nil {
		if errors.Is(err, solvers.RoutingNotSupported) {
			return reconcile.Result{}, nil
		}
		reqLogger.Error(err, "Invalid routing class for workspace")
		return reconcile.Result{}, r.markRoutingFailed(instance)
	}

	// Check if the WorkspaceRouting instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	if instance.GetDeletionTimestamp() != nil {
		reqLogger.Info("Finalizing WorkspaceRouting")
		return reconcile.Result{}, r.finalize(solver, instance)
	}

	if instance.Status.Phase == controllerv1alpha1.RoutingFailed {
		return reconcile.Result{}, nil
	}

	// Add finalizer for this CR if not already present
	if err := r.setFinalizer(reqLogger, solver, instance); err != nil {
		return reconcile.Result{}, err
	}

	workspaceMeta := solvers.WorkspaceMetadata{
		WorkspaceId:   instance.Spec.WorkspaceId,
		Namespace:     instance.Namespace,
		PodSelector:   instance.Spec.PodSelector,
		RoutingSuffix: instance.Spec.RoutingSuffix,
	}

	restrictedAccess, setRestrictedAccess := instance.Annotations[config.WorkspaceRestrictedAccessAnnotation]
	routingObjects, err := solver.GetSpecObjects(instance, workspaceMeta)
	if err != nil {
		var notReady *solvers.RoutingNotReady
		if errors.As(err, &notReady) {
			duration := notReady.Retry
			if duration.Milliseconds() == 0 {
				duration = 1 * time.Second
			}
			reqLogger.Info("controller not ready for workspace routing. Retrying", "DelayMs", duration.Milliseconds())
			return reconcile.Result{RequeueAfter: duration}, nil
		}

		var invalid *solvers.RoutingInvalid
		if errors.As(err, &invalid) {
			reqLogger.Error(invalid, "routing controller considers routing invalid")
			return reconcile.Result{}, r.markRoutingFailed(instance)
		}

		// generic error, just fail the reconciliation
		return reconcile.Result{}, err
	}

	services := routingObjects.Services
	for idx := range services {
		err := controllerutil.SetControllerReference(instance, &services[idx], r.Scheme)
		if err != nil {
			return reconcile.Result{}, err
		}
		if setRestrictedAccess {
			services[idx].Annotations = maputils.Append(services[idx].Annotations, config.WorkspaceRestrictedAccessAnnotation, restrictedAccess)
		}
	}
	ingresses := routingObjects.Ingresses
	for idx := range ingresses {
		err := controllerutil.SetControllerReference(instance, &ingresses[idx], r.Scheme)
		if err != nil {
			return reconcile.Result{}, err
		}
		if setRestrictedAccess {
			ingresses[idx].Annotations = maputils.Append(ingresses[idx].Annotations, config.WorkspaceRestrictedAccessAnnotation, restrictedAccess)
		}
	}
	routes := routingObjects.Routes
	for idx := range routes {
		err := controllerutil.SetControllerReference(instance, &routes[idx], r.Scheme)
		if err != nil {
			return reconcile.Result{}, err
		}
		if setRestrictedAccess {
			routes[idx].Annotations = maputils.Append(routes[idx].Annotations, config.WorkspaceRestrictedAccessAnnotation, restrictedAccess)
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
		return reconcile.Result{}, r.markRoutingFailed(instance)
	}

	return reconcile.Result{}, r.reconcileStatus(instance, routingObjects, exposedEndpoints, endpointsAreReady)
}

// setFinalizer ensures a finalizer is set on a workspaceRouting instance; no-op if finalizer is already present.
func (r *WorkspaceRoutingReconciler) setFinalizer(reqLogger logr.Logger, solver solvers.RoutingSolver, m *controllerv1alpha1.WorkspaceRouting) error {
	if !solver.FinalizerRequired(m) || contains(m.GetFinalizers(), workspaceRoutingFinalizer) {
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

func (r *WorkspaceRoutingReconciler) finalize(solver solvers.RoutingSolver, instance *controllerv1alpha1.WorkspaceRouting) error {
	if contains(instance.GetFinalizers(), workspaceRoutingFinalizer) {
		// let the solver finalize its stuff
		err := solver.Finalize(instance)
		if err != nil {
			return err
		}

		// Remove workspaceRoutingFinalizer. Once all finalizers have been
		// removed, the object will be deleted.
		instance.SetFinalizers(remove(instance.GetFinalizers(), workspaceRoutingFinalizer))
		err = r.Update(context.TODO(), instance)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *WorkspaceRoutingReconciler) markRoutingFailed(instance *controllerv1alpha1.WorkspaceRouting) error {
	instance.Status.Phase = controllerv1alpha1.RoutingFailed
	return r.Status().Update(context.TODO(), instance)
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
	bld := ctrl.NewControllerManagedBy(mgr).
		For(&controllerv1alpha1.WorkspaceRouting{}).
		Owns(&corev1.Service{}).
		Owns(&v1beta1.Ingress{})
	if config.ControllerCfg.IsOpenShift() {
		bld.Owns(&routeV1.Route{})
	}
	if r.SolverGetter == nil {
		return NoSolversEnabled
	}

	if err := r.SolverGetter.SetupControllerManager(bld); err != nil {
		return err
	}

	bld.WithEventFilter(getRoutingPredicatesForSolverFunc(r.SolverGetter))

	return bld.Complete(r)
}
