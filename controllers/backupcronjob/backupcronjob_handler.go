//
// Copyright (c) 2019-2025 Red Hat, Inc.
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
	"fmt"
	"time"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *BackupCronJobReconciler) getBackupJobPredicate() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			job, ok := e.ObjectNew.(*batchv1.Job)
			if !ok {
				return false
			}

			// Only reconcile if job related to DevWorkspace backup
			return job.Labels[constants.DevWorkspaceBackupJobLabel] == "true" &&
				job.Labels[constants.DevWorkspaceNameLabel] != ""
		},
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
}

func (r *BackupCronJobReconciler) getBackupJobEventHandler() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, object client.Object) []reconcile.Request {
		job, ok := object.(*batchv1.Job)
		if !ok {
			return []ctrl.Request{}
		}

		if err := r.handleBackupJobStatus(ctx, job); err != nil {
			r.Log.Error(err, "Failed to handle backup job status", "namespace", job.Namespace, "job", job.Name)
		}

		// Don't enqueue any reconcile requests for the main reconcile loop
		return []ctrl.Request{}
	})
}

// handleBackupJobStatus checks the status of a backup job and updates the corresponding DevWorkspace annotations.
func (r *BackupCronJobReconciler) handleBackupJobStatus(ctx context.Context, job *batchv1.Job) error {
	for _, condition := range job.Status.Conditions {
		if condition.Status != corev1.ConditionTrue {
			continue
		}

		if condition.Type != batchv1.JobComplete && condition.Type != batchv1.JobFailed {
			continue
		}

		devWorkspace, err := r.getWorkspaceFromJob(ctx, job)
		if err != nil {
			return err
		}

		switch condition.Type {
		case batchv1.JobComplete:
			return r.recordBackupSuccess(ctx, devWorkspace, condition)
		case batchv1.JobFailed:
			return r.recordBackupFailure(ctx, devWorkspace, condition)
		}
	}

	return nil
}

func (r *BackupCronJobReconciler) recordBackupSuccess(
	ctx context.Context,
	devWorkspace *dw.DevWorkspace,
	condition batchv1.JobCondition,
) error {
	origDevWorkspace := devWorkspace.DeepCopy()

	if devWorkspace.Annotations == nil {
		devWorkspace.Annotations = make(map[string]string)
	}

	devWorkspace.Annotations[constants.DevWorkspaceLastBackupSuccessfulAnnotation] = "true"
	devWorkspace.Annotations[constants.DevWorkspaceLastBackupFinishedAtAnnotation] = condition.LastTransitionTime.Format(time.RFC3339Nano)
	delete(devWorkspace.Annotations, constants.DevWorkspaceLastBackupErrorAnnotation)

	return r.Patch(ctx, devWorkspace, client.MergeFrom(origDevWorkspace))
}

func (r *BackupCronJobReconciler) recordBackupFailure(
	ctx context.Context,
	devWorkspace *dw.DevWorkspace,
	condition batchv1.JobCondition,
) error {
	origDevWorkspace := devWorkspace.DeepCopy()

	if devWorkspace.Annotations == nil {
		devWorkspace.Annotations = make(map[string]string)
	}

	// Truncate error message if it's too long (max 1024 chars for annotation values)
	errorMsg := condition.Message
	const maxLength = 1024
	if len(errorMsg) > maxLength {
		errorMsg = errorMsg[:maxLength-3] + "..."
	}

	devWorkspace.Annotations[constants.DevWorkspaceLastBackupSuccessfulAnnotation] = "false"
	devWorkspace.Annotations[constants.DevWorkspaceLastBackupFinishedAtAnnotation] = condition.LastTransitionTime.Format(time.RFC3339Nano)
	devWorkspace.Annotations[constants.DevWorkspaceLastBackupErrorAnnotation] = errorMsg

	return r.Patch(ctx, devWorkspace, client.MergeFrom(origDevWorkspace))
}

func (r *BackupCronJobReconciler) getWorkspaceFromJob(
	ctx context.Context,
	job *batchv1.Job,
) (*dw.DevWorkspace, error) {
	devWorkspaceName, ok := job.Labels[constants.DevWorkspaceNameLabel]
	if !ok || devWorkspaceName == "" {
		// Should not happen since we already checked this in the predicate
		return nil, fmt.Errorf("DevWorkspace name label not found for job %s in namespace %s", job.Name, job.Namespace)
	}

	devWorkspace := &dw.DevWorkspace{}
	devWorkspaceKey := types.NamespacedName{Name: devWorkspaceName, Namespace: job.Namespace}
	if err := r.Get(ctx, devWorkspaceKey, devWorkspace); err != nil {
		return nil, err
	}

	return devWorkspace, nil
}
