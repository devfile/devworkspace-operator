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

package controllers

import (
	"context"

	devworkspace "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"

	"github.com/devfile/devworkspace-operator/controllers/workspace/provision"
	"github.com/devfile/devworkspace-operator/pkg/provision/storage"

	"github.com/go-logr/logr"
	coputil "github.com/redhat-cop/operator-utils/pkg/util"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	storageCleanupFinalizer = "storage.controller.devfile.io"
)

func (r *DevWorkspaceReconciler) finalize(ctx context.Context, log logr.Logger, workspace *devworkspace.DevWorkspace) (reconcile.Result, error) {
	if !coputil.HasFinalizer(workspace, storageCleanupFinalizer) {
		return reconcile.Result{}, nil
	}
	workspace.Status.Message = "Cleaning up resources for deletion"
	err := r.Client.Status().Update(ctx, workspace)
	if err != nil && !k8sErrors.IsConflict(err) {
		return reconcile.Result{}, err
	}

	// Need to make sure Deployment is cleaned up before starting job to avoid mounting issues for RWO PVCs
	wait, err := provision.DeleteWorkspaceDeployment(ctx, workspace, r.Client)
	if err != nil {
		return reconcile.Result{}, err
	}
	if wait {
		return reconcile.Result{Requeue: true}, nil
	}

	terminating, err := r.namespaceIsTerminating(ctx, workspace.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	} else if terminating {
		// Namespace is terminating, it's redundant to clean PVC files since it's going to be removed
		log.Info("Namespace is terminating; clearing storage finalizer")
		coputil.RemoveFinalizer(workspace, storageCleanupFinalizer)
		return reconcile.Result{}, r.Update(ctx, workspace)
	}

	storageProvisioner, err := storage.GetProvisioner(workspace)
	if err != nil {
		log.Info("Failed to clean up DevWorkspace storage for %s: %s", workspace.Name, err)
		failedStatus := &currentStatus{
			Conditions: map[devworkspace.WorkspaceConditionType]string{
				"Error": err.Error(),
			},
			Phase: "Error",
		}
		return r.updateWorkspaceStatus(workspace, r.Log, failedStatus, reconcile.Result{}, nil)
	}
	err = storageProvisioner.CleanupWorkspaceStorage(workspace, provision.ClusterAPI{
		Ctx:    ctx,
		Client: r.Client,
		Scheme: r.Scheme,
		Logger: log,
	})
	if err != nil {
		switch storageErr := err.(type) {
		case *storage.NotReadyError:
			log.Info(storageErr.Message)
			return reconcile.Result{RequeueAfter: storageErr.RequeueAfter}, nil
		case *storage.ProvisioningError:
			log.Info("Failed to clean up DevWorkspace storage for %s: %s", workspace.Name, storageErr)
			failedStatus := &currentStatus{
				Conditions: map[devworkspace.WorkspaceConditionType]string{
					"Error": storageErr.Message,
				},
				Phase: "Error",
			}
			return r.updateWorkspaceStatus(workspace, r.Log, failedStatus, reconcile.Result{}, nil)
		default:
			return reconcile.Result{}, storageErr
		}
	}
	log.Info("PVC clean up successful; clearing finalizer")
	coputil.RemoveFinalizer(workspace, storageCleanupFinalizer)
	return reconcile.Result{}, r.Update(ctx, workspace)
}

func isFinalizerNecessary(workspace *devworkspace.DevWorkspace) bool {
	storageProvisioner, err := storage.GetProvisioner(workspace)
	if err != nil {
		// Return false here since storage isn't needed for invalid workspaces
		return false
	}
	return storageProvisioner.NeedsStorage(&workspace.Spec.Template)
}

func (r *DevWorkspaceReconciler) namespaceIsTerminating(ctx context.Context, namespace string) (bool, error) {
	namespacedName := types.NamespacedName{
		Name: namespace,
	}
	n := &corev1.Namespace{}

	err := r.Get(ctx, namespacedName, n)
	if err != nil {
		return false, err
	}

	return n.Status.Phase == corev1.NamespaceTerminating, nil
}
