//
// Copyright (c) 2019-2024 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package controllers

import (
	"context"

	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/conditions"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/dwerrors"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	"github.com/devfile/devworkspace-operator/pkg/provision/workspace/rbac"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/devfile/devworkspace-operator/pkg/provision/storage"
	wsprovision "github.com/devfile/devworkspace-operator/pkg/provision/workspace"
)

func (r *DevWorkspaceReconciler) workspaceNeedsFinalize(workspace *common.DevWorkspaceWithConfig) bool {
	for _, finalizer := range workspace.Finalizers {
		if finalizer == constants.StorageCleanupFinalizer {
			return true
		}
		if finalizer == constants.ServiceAccountCleanupFinalizer {
			return true
		}
	}
	return false
}

func (r *DevWorkspaceReconciler) finalize(ctx context.Context, log logr.Logger, workspace *common.DevWorkspaceWithConfig) (finalizeResult reconcile.Result, finalizeErr error) {
	// Tracked state for the finalize process; we update the workspace status in a deferred function (and pass the
	// named return value for finalize()) to update the workspace's status with whatever is in finalizeStatus
	// when this function returns.
	finalizeStatus := &currentStatus{phase: devworkspacePhaseTerminating}
	finalizeStatus.setConditionTrue(conditions.Started, "Cleaning up resources for deletion")
	defer func() (reconcile.Result, error) {
		if len(workspace.Finalizers) == 0 {
			// If there are no finalizers on the workspace, the workspace may be garbage collected before we get to update
			// its status. This avoids potentially logging a confusing error due to trying to set the status on a deleted
			// workspace. This check has to be in the deferred function since updateWorkspaceStatus will be called after the
			// client.Update() call that removes the last finalizer.
			return finalizeResult, finalizeErr
		}
		return r.updateWorkspaceStatus(workspace, log, finalizeStatus, finalizeResult, finalizeErr)
	}()

	for _, finalizer := range workspace.Finalizers {
		switch finalizer {
		case constants.StorageCleanupFinalizer:
			return r.finalizeStorage(ctx, log, workspace, finalizeStatus)
		case constants.ServiceAccountCleanupFinalizer:
			return r.finalizeServiceAccount(ctx, log, workspace, finalizeStatus)
		case constants.RBACCleanupFinalizer:
			return r.finalizeRBAC(ctx, log, workspace, finalizeStatus)
		}
	}
	return reconcile.Result{}, nil
}

func (r *DevWorkspaceReconciler) finalizeStorage(ctx context.Context, log logr.Logger, workspace *common.DevWorkspaceWithConfig, finalizeStatus *currentStatus) (reconcile.Result, error) {
	// Need to make sure Deployment is cleaned up before starting job to avoid mounting issues for RWO PVCs
	wait, err := wsprovision.DeleteWorkspaceDeployment(ctx, workspace, r.Client)
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
		controllerutil.RemoveFinalizer(workspace, constants.StorageCleanupFinalizer)
		return reconcile.Result{}, r.Update(ctx, workspace.DevWorkspace)
	}

	storageProvisioner, err := storage.GetProvisioner(workspace)
	if err != nil {
		log.Error(err, "Failed to clean up DevWorkspace storage")
		finalizeStatus.phase = dw.DevWorkspaceStatusError
		finalizeStatus.setConditionTrue(dw.DevWorkspaceError, err.Error())
		return reconcile.Result{}, nil
	}
	err = storageProvisioner.CleanupWorkspaceStorage(workspace, sync.ClusterAPI{
		Ctx:    ctx,
		Client: r.Client,
		Scheme: r.Scheme,
		Logger: log,
	})
	if err != nil {
		switch storageErr := err.(type) {
		case *dwerrors.RetryError:
			log.Info(storageErr.Message)
			return reconcile.Result{RequeueAfter: storageErr.RequeueAfter}, nil
		case *dwerrors.FailError:
			if workspace.Status.Phase != dw.DevWorkspaceStatusError {
				// Avoid repeatedly logging error unless it's relevant
				log.Error(storageErr, "Failed to clean up DevWorkspace storage")
			}
			finalizeStatus.phase = dw.DevWorkspaceStatusError
			finalizeStatus.setConditionTrue(dw.DevWorkspaceError, err.Error())
			return reconcile.Result{}, nil
		default:
			return reconcile.Result{}, storageErr
		}
	}
	log.Info("PVC clean up successful; clearing finalizer")
	controllerutil.RemoveFinalizer(workspace, constants.StorageCleanupFinalizer)
	return reconcile.Result{}, r.Update(ctx, workspace.DevWorkspace)
}

func (r *DevWorkspaceReconciler) finalizeRBAC(ctx context.Context, log logr.Logger, workspace *common.DevWorkspaceWithConfig, finalizeStatus *currentStatus) (reconcile.Result, error) {
	terminating, err := r.namespaceIsTerminating(ctx, workspace.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	} else if terminating {
		// Namespace is terminating, it's redundant to update roles/rolebindings since they will be removed with the workspace
		log.Info("Namespace is terminating; clearing RBAC finalizer")
		controllerutil.RemoveFinalizer(workspace, constants.RBACCleanupFinalizer)
		return reconcile.Result{}, r.Update(ctx, workspace.DevWorkspace)
	}

	if err := rbac.FinalizeRBAC(workspace, sync.ClusterAPI{
		Ctx:    ctx,
		Client: r.Client,
		Scheme: r.Scheme,
		Logger: log,
	}); err != nil {
		switch rbacErr := err.(type) {
		case *dwerrors.RetryError:
			log.Info(rbacErr.Error())
			return reconcile.Result{Requeue: true}, nil
		case *dwerrors.FailError:
			if workspace.Status.Phase != dw.DevWorkspaceStatusError {
				// Avoid repeatedly logging error unless it's relevant
				log.Error(rbacErr, "Failed to finalize workspace RBAC")
			}
			finalizeStatus.phase = dw.DevWorkspaceStatusError
			finalizeStatus.setConditionTrue(dw.DevWorkspaceError, err.Error())
			return reconcile.Result{}, nil
		default:
			return reconcile.Result{}, err
		}
	}
	log.Info("RBAC cleanup successful; clearing finalizer")
	controllerutil.RemoveFinalizer(workspace, constants.RBACCleanupFinalizer)
	return reconcile.Result{}, r.Update(ctx, workspace.DevWorkspace)
}

// Deprecated: Only required to support old workspaces that use the service account finalizer. The service account finalizer should
// not be added to new workspaces.
func (r *DevWorkspaceReconciler) finalizeServiceAccount(ctx context.Context, log logr.Logger, workspace *common.DevWorkspaceWithConfig, finalizeStatus *currentStatus) (reconcile.Result, error) {
	retry, err := wsprovision.FinalizeServiceAccount(workspace, ctx, r.NonCachingClient)
	if err != nil {
		log.Error(err, "Failed to finalize workspace ServiceAccount")
		finalizeStatus.phase = dw.DevWorkspaceStatusError
		finalizeStatus.setConditionTrue(dw.DevWorkspaceError, err.Error())
		return reconcile.Result{}, nil
	}
	if retry {
		return reconcile.Result{Requeue: true}, nil
	}
	log.Info("ServiceAccount clean up successful; clearing finalizer")
	controllerutil.RemoveFinalizer(workspace, constants.ServiceAccountCleanupFinalizer)
	return reconcile.Result{}, r.Update(ctx, workspace.DevWorkspace)
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
