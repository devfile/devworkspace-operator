//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package devworkspacerouting

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/devfile/devworkspace-operator/controllers/controller/devworkspacerouting/solvers"
	maputils "github.com/devfile/devworkspace-operator/internal/map"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	routeV1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
)

var (
	NoSolversEnabled = errors.New("reconciler does not define SolverGetter")
)

const devWorkspaceRoutingFinalizer = "devworkspacerouting.controller.devfile.io"

// DevWorkspaceRoutingReconciler reconciles a DevWorkspaceRouting object
type DevWorkspaceRoutingReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	// SolverGetter will be used to get solvers for a particular devWorkspaceRouting
	SolverGetter solvers.RoutingSolverGetter
}

// +kubebuilder:rbac:groups=controller.devfile.io,resources=devworkspaceroutings,verbs=*
// +kubebuilder:rbac:groups=controller.devfile.io,resources=devworkspaceroutings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=*
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=*
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=*
// +kubebuidler:rbac:groups=route.openshift.io,resources=routes/status,verbs=get,list,watch
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes/custom-host,verbs=create

func (r *DevWorkspaceRoutingReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)

	// Fetch the DevWorkspaceRouting instance
	instance := &controllerv1alpha1.DevWorkspaceRouting{}
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
	reqLogger = reqLogger.WithValues(constants.DevWorkspaceIDLoggerKey, instance.Spec.DevWorkspaceId)
	reqLogger.Info("Reconciling DevWorkspaceRouting")

	if instance.Spec.RoutingClass == "" {
		return reconcile.Result{}, r.markRoutingFailed(instance, "DevWorkspaceRouting requires field routingClass to be set")
	}

	solver, err := r.SolverGetter.GetSolver(r.Client, instance.Spec.RoutingClass)
	if err != nil {
		if errors.Is(err, solvers.RoutingNotSupported) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, r.markRoutingFailed(instance, fmt.Sprintf("Invalid routingClass for DevWorkspace: %s", err))
	}

	// Check if the DevWorkspaceRouting instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	if instance.GetDeletionTimestamp() != nil {
		reqLogger.Info("Finalizing DevWorkspaceRouting")
		return reconcile.Result{}, r.finalize(solver, instance)
	}

	if instance.Status.Phase == controllerv1alpha1.RoutingFailed {
		return reconcile.Result{}, nil
	}

	// Add finalizer for this CR if not already present
	if err := r.setFinalizer(reqLogger, solver, instance); err != nil {
		return reconcile.Result{}, err
	}

	workspaceMeta := solvers.DevWorkspaceMetadata{
		DevWorkspaceId: instance.Spec.DevWorkspaceId,
		Namespace:      instance.Namespace,
		PodSelector:    instance.Spec.PodSelector,
	}

	restrictedAccess, setRestrictedAccess := instance.Annotations[constants.DevWorkspaceRestrictedAccessAnnotation]
	routingObjects, err := solver.GetSpecObjects(instance, workspaceMeta)
	if err != nil {
		var notReady *solvers.RoutingNotReady
		if errors.As(err, &notReady) {
			duration := notReady.Retry
			if duration.Milliseconds() == 0 {
				duration = 1 * time.Second
			}
			reqLogger.Info("controller not ready for devworkspace routing. Retrying", "DelayMs", duration.Milliseconds())
			return reconcile.Result{RequeueAfter: duration}, r.reconcileStatus(instance, nil, nil, false, "Waiting for DevWorkspaceRouting controller to be ready")
		}

		var invalid *solvers.RoutingInvalid
		if errors.As(err, &invalid) {
			reqLogger.Error(invalid, "routing controller considers routing invalid")
			return reconcile.Result{}, r.markRoutingFailed(instance, fmt.Sprintf("Unable to provision networking for DevWorkspace: %s", invalid))
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
			services[idx].Annotations = maputils.Append(services[idx].Annotations, constants.DevWorkspaceRestrictedAccessAnnotation, restrictedAccess)
		}
	}
	ingresses := routingObjects.Ingresses
	for idx := range ingresses {
		err := controllerutil.SetControllerReference(instance, &ingresses[idx], r.Scheme)
		if err != nil {
			return reconcile.Result{}, err
		}
		if setRestrictedAccess {
			ingresses[idx].Annotations = maputils.Append(ingresses[idx].Annotations, constants.DevWorkspaceRestrictedAccessAnnotation, restrictedAccess)
		}
	}
	routes := routingObjects.Routes
	for idx := range routes {
		err := controllerutil.SetControllerReference(instance, &routes[idx], r.Scheme)
		if err != nil {
			return reconcile.Result{}, err
		}
		if setRestrictedAccess {
			routes[idx].Annotations = maputils.Append(routes[idx].Annotations, constants.DevWorkspaceRestrictedAccessAnnotation, restrictedAccess)
		}
	}

	servicesInSync, clusterServices, err := r.syncServices(instance, services)
	if err != nil {
		reqLogger.Error(err, "Error syncing services")
		return reconcile.Result{Requeue: true}, r.reconcileStatus(instance, nil, nil, false, "Preparing services")
	} else if !servicesInSync {
		reqLogger.Info("Services not in sync")
		return reconcile.Result{Requeue: true}, r.reconcileStatus(instance, nil, nil, false, "Preparing services")
	}

	clusterRoutingObj := solvers.RoutingObjects{
		Services: clusterServices,
	}

	if infrastructure.IsOpenShift() {
		routesInSync, clusterRoutes, err := r.syncRoutes(instance, routes)
		if err != nil {
			reqLogger.Error(err, "Error syncing routes")
			return reconcile.Result{Requeue: true}, r.reconcileStatus(instance, nil, nil, false, "Preparing routes")
		} else if !routesInSync {
			reqLogger.Info("Routes not in sync")
			return reconcile.Result{Requeue: true}, r.reconcileStatus(instance, nil, nil, false, "Preparing routes")
		}
		clusterRoutingObj.Routes = clusterRoutes
	} else {
		ingressesInSync, clusterIngresses, err := r.syncIngresses(instance, ingresses)
		if err != nil {
			reqLogger.Error(err, "Error syncing ingresses")
			return reconcile.Result{Requeue: true}, r.reconcileStatus(instance, nil, nil, false, "Preparing ingresses")
		} else if !ingressesInSync {
			reqLogger.Info("Ingresses not in sync")
			return reconcile.Result{Requeue: true}, r.reconcileStatus(instance, nil, nil, false, "Preparing ingresses")
		}
		clusterRoutingObj.Ingresses = clusterIngresses
	}

	exposedEndpoints, endpointsAreReady, err := solver.GetExposedEndpoints(instance.Spec.Endpoints, clusterRoutingObj)
	if err != nil {
		reqLogger.Error(err, "Could not get exposed endpoints for devworkspace")
		return reconcile.Result{}, r.markRoutingFailed(instance, fmt.Sprintf("Could not get exposed endpoints for DevWorkspace: %s", err))
	}

	return reconcile.Result{}, r.reconcileStatus(instance, &routingObjects, exposedEndpoints, endpointsAreReady, "")
}

// setFinalizer ensures a finalizer is set on a devWorkspaceRouting instance; no-op if finalizer is already present.
func (r *DevWorkspaceRoutingReconciler) setFinalizer(reqLogger logr.Logger, solver solvers.RoutingSolver, m *controllerv1alpha1.DevWorkspaceRouting) error {
	if !solver.FinalizerRequired(m) || contains(m.GetFinalizers(), devWorkspaceRoutingFinalizer) {
		return nil
	}

	reqLogger.Info("Adding Finalizer for the DevWorkspaceRouting")
	m.SetFinalizers(append(m.GetFinalizers(), devWorkspaceRoutingFinalizer))

	// Update CR
	err := r.Update(context.TODO(), m)
	if err != nil {
		reqLogger.Error(err, "Failed to update DevWorkspaceRouting with finalizer")
		return err
	}
	return nil
}

func (r *DevWorkspaceRoutingReconciler) finalize(solver solvers.RoutingSolver, instance *controllerv1alpha1.DevWorkspaceRouting) error {
	if contains(instance.GetFinalizers(), devWorkspaceRoutingFinalizer) {
		// let the solver finalize its stuff
		err := solver.Finalize(instance)
		if err != nil {
			return err
		}

		// Remove devWorkspaceRoutingFinalizer. Once all finalizers have been
		// removed, the object will be deleted.
		instance.SetFinalizers(remove(instance.GetFinalizers(), devWorkspaceRoutingFinalizer))
		err = r.Update(context.TODO(), instance)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *DevWorkspaceRoutingReconciler) markRoutingFailed(instance *controllerv1alpha1.DevWorkspaceRouting, message string) error {
	instance.Status.Message = message
	instance.Status.Phase = controllerv1alpha1.RoutingFailed
	return r.Status().Update(context.TODO(), instance)
}

func (r *DevWorkspaceRoutingReconciler) reconcileStatus(
	instance *controllerv1alpha1.DevWorkspaceRouting,
	routingObjects *solvers.RoutingObjects,
	exposedEndpoints map[string]controllerv1alpha1.ExposedEndpointList,
	endpointsReady bool,
	message string) error {

	if !endpointsReady {
		instance.Status.Phase = controllerv1alpha1.RoutingPreparing
		instance.Status.Message = message
		return r.Status().Update(context.TODO(), instance)
	}
	if instance.Status.Phase == controllerv1alpha1.RoutingReady &&
		cmp.Equal(instance.Status.PodAdditions, routingObjects.PodAdditions) &&
		cmp.Equal(instance.Status.ExposedEndpoints, exposedEndpoints) {
		return nil
	}
	instance.Status.Phase = controllerv1alpha1.RoutingReady
	instance.Status.Message = "DevWorkspaceRouting prepared"
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

func (r *DevWorkspaceRoutingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	maxConcurrentReconciles, err := config.GetMaxConcurrentReconciles()
	if err != nil {
		return err
	}

	bld := ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}).
		For(&controllerv1alpha1.DevWorkspaceRouting{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{})
	if infrastructure.IsOpenShift() {
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
