//
// Copyright (c) 2019-2021 Red Hat, Inc.
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

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"

	"github.com/go-logr/logr"
	coputil "github.com/redhat-cop/operator-utils/pkg/util"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/devfile/devworkspace-operator/pkg/provision/storage"
	wsprovision "github.com/devfile/devworkspace-operator/pkg/provision/workspace"
)

const (
	storageCleanupFinalizer = "storage.controller.devfile.io"
)

func (r *DevWorkspaceReconciler) finalize(ctx context.Context, log logr.Logger, workspace *dw.DevWorkspace) (reconcile.Result, error) {
	if !coputil.HasFinalizer(workspace, storageCleanupFinalizer) {
		return reconcile.Result{}, nil
	}
	workspace.Status.Message = "Cleaning up resources for deletion"
	workspace.Status.Phase = devworkspacePhaseTerminating
	err := r.Client.Status().Update(ctx, workspace)
	if err != nil && !k8sErrors.IsConflict(err) {
		return reconcile.Result{}, err
	}

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
		coputil.RemoveFinalizer(workspace, storageCleanupFinalizer)
		return reconcile.Result{}, r.Update(ctx, workspace)
	}

	storageProvisioner, err := storage.GetProvisioner(workspace)
	if err != nil {
		log.Error(err, "Failed to clean up DevWorkspace storage")
		failedStatus := currentStatus{phase: dw.DevWorkspaceStatusError}
		failedStatus.setConditionTrue(dw.DevWorkspaceError, err.Error())
		return r.updateWorkspaceStatus(workspace, r.Log, &failedStatus, reconcile.Result{}, nil)
	}
	err = storageProvisioner.CleanupWorkspaceStorage(workspace, wsprovision.ClusterAPI{
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
			log.Error(storageErr, "Failed to clean up DevWorkspace storage")
			failedStatus := currentStatus{phase: dw.DevWorkspaceStatusError}
			failedStatus.setConditionTrue(dw.DevWorkspaceError, err.Error())
			return r.updateWorkspaceStatus(workspace, r.Log, &failedStatus, reconcile.Result{}, nil)
		default:
			return reconcile.Result{}, storageErr
		}
	}
	log.Info("PVC clean up successful; clearing finalizer")
	coputil.RemoveFinalizer(workspace, storageCleanupFinalizer)
	return reconcile.Result{}, r.Update(ctx, workspace)
}

func isFinalizerNecessary(workspace *dw.DevWorkspace, provisioner storage.Provisioner) bool {
	return provisioner.NeedsStorage(&workspace.Spec.Template)
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
