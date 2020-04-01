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

package component

import (
	"context"
	"fmt"
	"github.com/che-incubator/che-workspace-operator/pkg/adaptor"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	workspacev1alpha1 "github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_component")

var configMapDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta", "ObjectMeta"),
}

// Add creates a new Component Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileComponent{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("component-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Component
	err = c.Watch(&source.Kind{Type: &workspacev1alpha1.Component{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &workspacev1alpha1.Component{},
	})

	return nil
}

// blank assignment to verify that ReconcileComponent implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileComponent{}

// ReconcileComponent reconciles a Component object
type ReconcileComponent struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Component object and makes changes based on the state read
// and what is in the Component.Spec
func (r *ReconcileComponent) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Component")

	// Fetch the Component instance
	instance := &workspacev1alpha1.Component{}
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

	var components []workspacev1alpha1.ComponentDescription
	dockerimageDevfileComponents, pluginDevfileComponents, err := adaptor.SortComponentsByType(instance.Spec.Components)
	if err != nil {
		return reconcile.Result{}, err
	}

	commands := instance.Spec.Commands

	dockerimageComponents, err := adaptor.AdaptDockerimageComponents(instance.Spec.WorkspaceId, dockerimageDevfileComponents, commands)
	if err != nil {
		reqLogger.Info("Failed to adapt dockerimage components")
		return reconcile.Result{}, err
	}
	components = append(components, dockerimageComponents...)

	pluginComponents, brokerConfigMap, err := adaptor.AdaptPluginComponents(instance.Spec.WorkspaceId, instance.Namespace, pluginDevfileComponents)
	if err != nil {
		reqLogger.Info("Failed to adapt plugin components")
		return reconcile.Result{}, err
	}
	components = append(components, pluginComponents...)

	if brokerConfigMap != nil {
		reqLogger.Info("Reconciling broker ConfigMap")
		ok, err := r.reconcileConfigMap(instance, brokerConfigMap, reqLogger)
		if err != nil {
			return reconcile.Result{}, err
		}
		if !ok {
			return reconcile.Result{Requeue: true}, nil
		}
	}

	return reconcile.Result{}, r.reconcileStatus(instance, components)
}

func (r *ReconcileComponent) reconcileConfigMap(instance *workspacev1alpha1.Component, cm *corev1.ConfigMap, log logr.Logger) (ok bool, err error) {
	err = controllerutil.SetControllerReference(instance, cm, r.scheme)
	if err != nil {
		return false, err
	}
	clusterConfigMap := &corev1.ConfigMap{}
	namespacedName := types.NamespacedName{
		Namespace: cm.Namespace,
		Name:      cm.Name,
	}
	err = r.client.Get(context.TODO(), namespacedName, clusterConfigMap)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating broker ConfigMap")
			err := r.client.Create(context.TODO(), cm)
			return false, err
		}
		return false, err
	}

	if !cmp.Equal(cm, clusterConfigMap, configMapDiffOpts) {
		log.Info("Updating broker ConfigMap")
		log.V(2).Info(fmt.Sprintf("Diff: %s\n", cmp.Diff(cm, clusterConfigMap, configMapDiffOpts)))
		clusterConfigMap.Data = cm.Data
		err := r.client.Update(context.TODO(), clusterConfigMap)
		return false, err
	}

	return true, nil
}

func (r *ReconcileComponent) reconcileStatus(instance *workspacev1alpha1.Component, components []workspacev1alpha1.ComponentDescription) error {
	if instance.Status.Ready && cmp.Equal(instance.Status.ComponentDescriptions, components) {
		return nil
	}
	instance.Status.ComponentDescriptions = components
	instance.Status.Ready = true
	return r.client.Status().Update(context.TODO(), instance)
}
