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
	"errors"
	"fmt"

	"github.com/devfile/devworkspace-operator/pkg/adaptor"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/library"
)

var configMapDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta", "ObjectMeta"),
}

// ComponentReconciler reconciles a Component object
type ComponentReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=controller.devfile.io,resources=components,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=controller.devfile.io,resources=components/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=configmap,verbs=get;list;watch;create;update;patch;delete

func (r *ComponentReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling Component")

	// Fetch the Component instance
	instance := &controllerv1alpha1.Component{}
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

	if instance.DeletionTimestamp != nil {
		reqLogger.V(5).Info("Skipping reconcile of deleted resource")
		return reconcile.Result{}, nil
	}

	initContainers, mainComponents, err := library.GetInitContainers(devworkspace.DevWorkspaceTemplateSpecContent{
		Components: instance.Spec.Components,
		Commands:   instance.Spec.Commands,
		Events:     instance.Spec.Events,
	})
	if err != nil {
		return reconcile.Result{}, err
	}

	var components []controllerv1alpha1.ComponentDescription
	dockerimageDevfileComponents, pluginDevfileComponents, err := adaptor.SortComponentsByType(mainComponents)
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
		var downloadErr *adaptor.DownloadMetasError
		if errors.As(err, &downloadErr) {
			reqLogger.Info("Failed to download plugin metas", "err", downloadErr.Unwrap())
			return reconcile.Result{}, r.FailComponent(instance, downloadErr.Error())
		} else {
			reqLogger.Info("Failed to adapt plugin components")
			return reconcile.Result{}, err
		}
	}
	components = append(components, pluginComponents...)

	initContainerComponents, err := adaptor.AdaptInitContainerComponents(instance.Spec.WorkspaceId, initContainers)
	if err != nil {
		return reconcile.Result{}, err
	}

	components = append(components, initContainerComponents...)

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

func (r *ComponentReconciler) reconcileConfigMap(instance *controllerv1alpha1.Component, cm *corev1.ConfigMap, log logr.Logger) (ok bool, err error) {
	err = controllerutil.SetControllerReference(instance, cm, r.Scheme)
	if err != nil {
		return false, err
	}
	clusterConfigMap := &corev1.ConfigMap{}
	namespacedName := types.NamespacedName{
		Namespace: cm.Namespace,
		Name:      cm.Name,
	}
	err = r.Get(context.TODO(), namespacedName, clusterConfigMap)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			log.Info("Creating broker ConfigMap")
			err := r.Create(context.TODO(), cm)
			return false, err
		}
		return false, err
	}

	if !cmp.Equal(cm, clusterConfigMap, configMapDiffOpts) {
		log.Info("Updating broker ConfigMap")
		log.V(2).Info(fmt.Sprintf("Diff: %s\n", cmp.Diff(cm, clusterConfigMap, configMapDiffOpts)))
		clusterConfigMap.Data = cm.Data
		err := r.Update(context.TODO(), clusterConfigMap)
		return false, err
	}

	return true, nil
}

func (r *ComponentReconciler) reconcileStatus(instance *controllerv1alpha1.Component, components []controllerv1alpha1.ComponentDescription) error {
	if instance.Status.Ready && cmp.Equal(instance.Status.ComponentDescriptions, components) {
		return nil
	}
	instance.Status.ComponentDescriptions = components
	instance.Status.Ready = true
	return r.Status().Update(context.TODO(), instance)
}

func (r *ComponentReconciler) FailComponent(instance *controllerv1alpha1.Component, message string) error {
	instance.Status.Failed = true
	instance.Status.Message = message
	return r.Status().Update(context.TODO(), instance)
}

func (r *ComponentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&controllerv1alpha1.Component{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
