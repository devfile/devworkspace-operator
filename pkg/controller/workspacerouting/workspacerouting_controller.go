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
	"github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/model"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	workspacev1alpha1 "github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
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
	return &ReconcileWorkspaceRouting{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		solvers: map[string]WorkspaceRoutingSolver{
			model.RoutingTypeDefault: &BasicSolver{
				Client: mgr.GetClient(),
			},
			model.RoutingTypeOpenShiftOauth: &OpenshiftOAuthSolver{
				Client: mgr.GetClient(),
			},
		},
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("workspacerouting-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource WorkspaceRouting
	err = c.Watch(&source.Kind{Type: &workspacev1alpha1.WorkspaceRouting{}}, &handler.EnqueueRequestForObject{}, predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.MetaOld == nil {
				log.Error(nil, "UpdateEvent has no old metadata", "event", e)
				return false
			}
			if e.ObjectOld == nil {
				log.Error(nil, "UpdateEvent has no old runtime object to update", "event", e)
				return false
			}
			if e.ObjectNew == nil {
				log.Error(nil, "UpdateEvent has no new runtime object for update", "event", e)
				return false
			}
			if e.MetaNew == nil {
				log.Error(nil, "UpdateEvent has no new metadata", "event", e)
				return false
			}
			if e.MetaNew.GetGeneration() == e.MetaOld.GetGeneration() {
				return false
			}
			return true
		},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileWorkspaceRouting implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileWorkspaceRouting{}

// ReconcileWorkspaceRouting reconciles a WorkspaceRouting object
type ReconcileWorkspaceRouting struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client  client.Client
	scheme  *runtime.Scheme
	solvers map[string]WorkspaceRoutingSolver
}

type CurrentReconcile struct {
	Instance  *workspacev1alpha1.WorkspaceRouting
	ReqLogger logr.Logger
	Reconcile *ReconcileWorkspaceRouting
	Solver    WorkspaceRoutingSolver
}

func (r *ReconcileWorkspaceRouting) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

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

	solver, found := r.solvers[instance.Spec.RoutingClass]
	if !found {
		reqLogger.Info("Reconciling Skipped: unsupported routing class", "routing", instance.Spec.RoutingClass)
		return reconcile.Result{}, err
	}

	reqLogger = reqLogger.WithValues("RoutingClass", instance.Spec.RoutingClass)

	reqLogger.V(1).Info("Reconciling", "expected", instance.Spec.Exposed, "phase", instance.Status.Phase)

	currentReconcile := CurrentReconcile{
		Instance:  instance,
		ReqLogger: reqLogger,
		Reconcile: r,
		Solver:    solver,
	}

	switch instance.Spec.Exposed {
	case true:
		switch instance.Status.Phase {

		case "", workspacev1alpha1.WorkspaceRoutingHidden:
			result, err := solver.CreateOrUpdateRoutingObjects(currentReconcile)
			return updatePhaseIfSuccess(currentReconcile, result, err, workspacev1alpha1.WorkspaceRoutingExposing)

		case workspacev1alpha1.WorkspaceRoutingExposing:
			nextPhase, result, err := solver.CheckRoutingObjects(currentReconcile, workspacev1alpha1.WorkspaceRoutingExposed)
			return updatePhaseIfSuccess(currentReconcile, result, err, nextPhase)

		case workspacev1alpha1.WorkspaceRoutingExposed:
			result, err := updateExposedEndpoints(currentReconcile)
			return updatePhaseIfSuccess(currentReconcile, result, err, workspacev1alpha1.WorkspaceRoutingReady)

		case workspacev1alpha1.WorkspaceRoutingReady:
			return reconcile.Result{}, nil

		case workspacev1alpha1.WorkspaceRoutingFailed:
			return reconcile.Result{}, nil

		case workspacev1alpha1.WorkspaceRoutingHiding:
			nextPhase, result, err := solver.CheckRoutingObjects(currentReconcile, workspacev1alpha1.WorkspaceRoutingHidden)
			return updatePhaseIfSuccess(currentReconcile, result, err, nextPhase)
		}
	case false:
		switch instance.Status.Phase {
		case "":
			return updatePhaseIfSuccess(currentReconcile, reconcile.Result{}, nil, workspacev1alpha1.WorkspaceRoutingHidden)

		case workspacev1alpha1.WorkspaceRoutingHidden:
			return reconcile.Result{}, nil

		case workspacev1alpha1.WorkspaceRoutingExposing, workspacev1alpha1.WorkspaceRoutingExposed:
			result, err := solver.DeleteRoutingObjects(currentReconcile)
			return updatePhaseIfSuccess(currentReconcile, result, err, workspacev1alpha1.WorkspaceRoutingHiding)

		case workspacev1alpha1.WorkspaceRoutingReady:
			result, err := cleanExposedEndpoints(currentReconcile)
			return updatePhaseIfSuccess(currentReconcile, result, err, workspacev1alpha1.WorkspaceRoutingExposed)

		case workspacev1alpha1.WorkspaceRoutingHiding:
			nextPhase, result, err := solver.CheckRoutingObjects(currentReconcile, workspacev1alpha1.WorkspaceRoutingHidden)
			return updatePhaseIfSuccess(currentReconcile, result, err, nextPhase)

		case workspacev1alpha1.WorkspaceRoutingFailed:
			result, err := solver.DeleteRoutingObjects(currentReconcile)
			if err != nil {
				result, err = cleanExposedEndpoints(currentReconcile)
			}
			return updatePhaseIfSuccess(currentReconcile, result, err, workspacev1alpha1.WorkspaceRoutingHiding)
		}
	}
	return reconcile.Result{}, nil
}

func updatePhaseIfSuccess(cr CurrentReconcile, result reconcile.Result, err error, nextPhase workspacev1alpha1.WorkspaceRoutingPhase) (reconcile.Result, error) {
	existingPhase := cr.Instance.Status.Phase
	updateWhileConflict := func(action func() error) error {
		for {
			err := action()
			if !errors.IsConflict(err) {
				return err
			}
			if err2 := cr.Reconcile.client.Get(context.TODO(),
				types.NamespacedName{
					Namespace: cr.Instance.Namespace,
					Name:      cr.Instance.Name,
				},
				cr.Instance,
			); err2 != nil && !errors.IsNotFound(err) {
				cr.ReqLogger.Error(err, "When trying to get workspace routing "+cr.Instance.Name)
				return err
			}
		}
	}

	if err != nil {
		updateError := updateWhileConflict(func() error {
			cr.Instance.Status.Phase = workspacev1alpha1.WorkspaceRoutingFailed
			return cr.Reconcile.client.Status().Update(context.TODO(), cr.Instance)
		})
		if updateError != nil {
			cr.ReqLogger.Error(err, "When trying to update the status phase to: "+string(workspacev1alpha1.WorkspaceRoutingFailed))
		}
		return result, err
	}
	updateError := updateWhileConflict(func() error {
		cr.Instance.Status.Phase = nextPhase
		return cr.Reconcile.client.Status().Update(context.TODO(), cr.Instance)
	})
	if updateError != nil {
		cr.ReqLogger.Error(err, "When trying to update the status phase to: "+string(nextPhase))
	}
	if existingPhase != cr.Instance.Status.Phase {
		cr.ReqLogger.Info("Phase: " + string(existingPhase) + " => " + string(cr.Instance.Status.Phase))
	}
	return reconcile.Result{Requeue: true}, err
}

func cleanExposedEndpoints(cr CurrentReconcile) (reconcile.Result, error) {
	cr.Instance.Status.ExposedEndpoints = map[string]workspacev1alpha1.ExposedEndpointList{}
	err := cr.Reconcile.client.Status().Update(context.TODO(), cr.Instance)
	if err != nil {
		log.Error(err, "When updating the routing status with no endpoints")
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func updateExposedEndpoints(cr CurrentReconcile) (reconcile.Result, error) {
	cr.Instance.Status.ExposedEndpoints = cr.Solver.BuildExposedEndpoints(cr)
	err := cr.Reconcile.client.Status().Update(context.TODO(), cr.Instance)
	if err != nil {
		log.Error(err, "When updating the routing status with exposed endpoints", "exposedEndpoints", cr.Instance.Status.ExposedEndpoints)
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

type WorkspaceRoutingSolver interface {
	CreateOrUpdateRoutingObjects(currentReconcile CurrentReconcile) (reconcile.Result, error)
	CheckRoutingObjects(currentReconcile CurrentReconcile, targetPhase workspacev1alpha1.WorkspaceRoutingPhase) (workspacev1alpha1.WorkspaceRoutingPhase, reconcile.Result, error)
	BuildExposedEndpoints(currentReconcile CurrentReconcile) map[string]workspacev1alpha1.ExposedEndpointList
	DeleteRoutingObjects(currentReconcile CurrentReconcile) (reconcile.Result, error)
}

func DeleteRoutingObjects(cr CurrentReconcile, objectTypes []runtime.Object) (reconcile.Result, error) {
	cr.ReqLogger.Info("Deleting K8s objects")
	for _, list := range objectTypes {
		cr.Reconcile.client.List(context.TODO(), list,
			client.InNamespace(cr.Instance.Namespace),
			client.MatchingLabels{
				"org.eclipse.che.workspace.routing.workspace_id": cr.Instance.Name,
			})
		items := reflect.ValueOf(list).Elem().FieldByName("Items")
		if !items.IsValid() {
			return reconcile.Result{}, nil
		}
		for i := 0; i < items.Len(); i++ {
			item := items.Index(i).Addr().Interface()
			if itemMeta, isMeta := item.(metav1.Object); isMeta {
				if itemRuntime, isRuntime := item.(runtime.Object); isRuntime {
					log.Info("  => Deleting "+reflect.TypeOf(itemRuntime).Elem().String(), "name", itemMeta.GetName())
					err := cr.Reconcile.client.Delete(context.TODO(), itemRuntime)
					if err != nil {
						cr.ReqLogger.Error(err, "Error when creating K8S object own by the Workspace Routing: ", "k8sObject", itemRuntime)
						return reconcile.Result{}, err
					}
				}
			}
		}
	}
	return reconcile.Result{}, nil
}

func CreateOrUpdate(cr CurrentReconcile, k8sObjects []runtime.Object, diffOpts cmp.Options, replaceFun func(found runtime.Object, new runtime.Object)) (reconcile.Result, error) {
	cr.ReqLogger.Info("Creating K8s objects")
	reqLogger := cr.ReqLogger
	instance := cr.Instance
	r := cr.Reconcile

	for _, k8sObject := range k8sObjects {
		k8sObjectAsMetaObject, isMeta := k8sObject.(metav1.Object)
		if !isMeta {
			return reconcile.Result{}, errors.NewBadRequest("Converted objects are not valid K8s objects")
		}

		reqLogger.V(1).Info("  - Managing K8s Object", "kind", reflect.TypeOf(k8sObject).Elem().String(), "name", k8sObjectAsMetaObject.GetName())

		// Set Workspace instance as the owner and controller
		if err := controllerutil.SetControllerReference(instance, k8sObjectAsMetaObject, r.scheme); err != nil {
			reqLogger.Error(err, "Error when setting controller reference")
			return reconcile.Result{}, err
		}

		// Set Workspace instance as the owner and controller
		k8sObjectAsMetaObject.SetLabels(map[string]string{
			"org.eclipse.che.workspace.routing.workspace_id": instance.Name,
		})

		// Check if the k8s Object already exists

		found := reflect.New(reflect.TypeOf(k8sObject).Elem()).Interface().(runtime.Object)
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: k8sObjectAsMetaObject.GetName(), Namespace: k8sObjectAsMetaObject.GetNamespace()}, found)
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Info("  => Creating "+reflect.TypeOf(k8sObjectAsMetaObject).Elem().String(), "name", k8sObjectAsMetaObject.GetName())
			err = r.client.Create(context.TODO(), k8sObject)
			if err != nil {
				reqLogger.Error(err, "Error when creating K8S object: ", "k8sObject", k8sObject)
				return reconcile.Result{}, err
			}
			continue
		} else if err != nil {
			reqLogger.Error(err, "Error when getting K8S object: ", "k8sObject", k8sObjectAsMetaObject)
			return reconcile.Result{}, err
		}

		r.scheme.Default(k8sObject)

		// Update the found object and write the result back if there are any changes

		foundSpecValue := reflect.ValueOf(found).Elem().FieldByName("Spec")
		k8sObjectSpecValue := reflect.ValueOf(k8sObject).Elem().FieldByName("Spec")

		var foundToUse interface{} = found
		var newToUse interface{} = k8sObject
		if foundSpecValue.IsValid() {
			foundToUse = foundSpecValue.Interface()
			newToUse = k8sObjectSpecValue.Interface()
		}

		if !cmp.Equal(foundToUse, newToUse, diffOpts) {
			reqLogger.V(1).Info("  => Differences: " + cmp.Diff(foundToUse, newToUse, diffOpts...))
			replaceFun(found, k8sObject)
			reqLogger.Info("  => Updating "+reflect.TypeOf(k8sObjectAsMetaObject).Elem().String(), "name", k8sObjectAsMetaObject.GetName())
			err = r.client.Update(context.TODO(), found)
			if err != nil {
				reqLogger.Error(err, "Error when updating K8S object: ", "k8sObject", k8sObjectAsMetaObject)
				return reconcile.Result{}, err
			}
		}
	}

	return reconcile.Result{}, nil
}
