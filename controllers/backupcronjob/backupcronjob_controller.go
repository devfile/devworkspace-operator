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

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/conditions"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/go-logr/logr"
	"github.com/robfig/cron/v3"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	defaultBackupImage = "registry.access.redhat.com/ubi8/ubi-minimal:latest"
)

// BackupCronJobReconciler reconciles `BackupCronJob` configuration for the purpose of backing up workspace PVCs.
type BackupCronJobReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	cron *cron.Cron
}

// shouldReconcileOnUpdate determines whether the BackupCronJobReconciler should reconcile
// based on changes in the DevWorkspaceOperatorConfig object.
func shouldReconcileOnUpdate(e event.UpdateEvent, log logr.Logger) bool {
	log.Info("DevWorkspaceOperatorConfig update event received")
	oldConfig, ok := e.ObjectOld.(*controllerv1alpha1.DevWorkspaceOperatorConfig)
	if !ok {
		return false
	}
	newConfig, ok := e.ObjectNew.(*controllerv1alpha1.DevWorkspaceOperatorConfig)
	if !ok {
		return false
	}

	oldBackup := oldConfig.Config.Workspace.BackupCronJob
	newBackup := newConfig.Config.Workspace.BackupCronJob

	differentBool := func(a, b *bool) bool {
		switch {
		case a == nil && b == nil:
			return false
		case a == nil || b == nil:
			return true
		default:
			return *a != *b
		}
	}

	if oldBackup == nil && newBackup == nil {
		return false
	}
	if (oldBackup == nil && newBackup != nil) || (oldBackup != nil && newBackup == nil) {
		return true
	}
	if differentBool(oldBackup.Enable, newBackup.Enable) {
		return true
	}

	if oldBackup.Schedule != newBackup.Schedule {
		return true
	}

	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupCronJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	log := r.Log.WithName("setupWithManager")
	log.Info("Setting up BackupCronJobReconciler")

	configPredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return shouldReconcileOnUpdate(e, log)
		},
		CreateFunc:  func(e event.CreateEvent) bool { return true },
		DeleteFunc:  func(e event.DeleteEvent) bool { return true },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}

	r.cron = cron.New()

	return ctrl.NewControllerManagedBy(mgr).
		Named("BackupCronJob").
		Watches(&controllerv1alpha1.DevWorkspaceOperatorConfig{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, object client.Object) []reconcile.Request {
				operatorNamespace, err := infrastructure.GetNamespace()
				// Ignore events from other namespaces
				if err != nil || object.GetNamespace() != operatorNamespace || object.GetName() != config.OperatorConfigName {
					log.Info("Received event from different namespace, ignoring", "namespace", object.GetNamespace())
					return []ctrl.Request{}
				}

				return []ctrl.Request{
					{
						NamespacedName: client.ObjectKey{
							Name:      object.GetName(),
							Namespace: object.GetNamespace(),
						},
					},
				}
			}),
		).
		WithEventFilter(configPredicate).
		Complete(r)
}

// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list
// +kubebuilder:rbac:groups=controller.devfile.io,resources=devworkspaceoperatorconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=workspace.devfile.io,resources=devworkspaces,verbs=get;list
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list

// Reconcile is the main reconciliation loop for the BackupCronJob controller.
func (r *BackupCronJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log
	log.Info("Reconciling BackupCronJob", "DWOC", req.NamespacedName)

	dwOperatorConfig := &controllerv1alpha1.DevWorkspaceOperatorConfig{}
	err := r.Get(ctx, req.NamespacedName, dwOperatorConfig)
	if err != nil {
		log.Error(err, "Failed to get DevWorkspaceOperatorConfig")
		r.stopCron(log)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	isCronConfigured := r.isBackupEnabled(dwOperatorConfig)
	if !isCronConfigured {
		log.Info("DevWorkspace backup is not configured, stopping cron scheduler and skipping reconciliation")
		r.stopCron(log)
		return ctrl.Result{}, nil
	}

	backUpConfig := dwOperatorConfig.Config.Workspace.BackupCronJob
	r.startCron(ctx, backUpConfig, log)

	return ctrl.Result{}, nil
}

// isBackupEnabled checks if the backup cron job is enabled in the configuration.
func (r *BackupCronJobReconciler) isBackupEnabled(config *controllerv1alpha1.DevWorkspaceOperatorConfig) bool {
	if config.Config != nil && config.Config.Workspace != nil && config.Config.Workspace.BackupCronJob != nil {
		if config.Config.Workspace.BackupCronJob.Enable != nil && *config.Config.Workspace.BackupCronJob.Enable {
			return true
		}
	}
	return false
}

// startCron starts the cron scheduler with the backup job according to the provided configuration.
func (r *BackupCronJobReconciler) startCron(ctx context.Context, backUpConfig *controllerv1alpha1.BackupCronJobConfig, logger logr.Logger) {
	log := logger.WithName("backup cron")
	log.Info("Starting backup cron scheduler")

	// remove existing cronjob tasks
	// we cannot update the existing tasks, so we need to remove them and add new ones
	entries := r.cron.Entries()
	for _, entry := range entries {
		log.Info("Removing existing cronjob task", "entryID", entry.ID)
		r.cron.Remove(entry.ID)
	}

	// add cronjob task
	log.Info("Adding cronjob task", "schedule", backUpConfig.Schedule)
	_, err := r.cron.AddFunc(backUpConfig.Schedule, func() {
		taskLog := logger.WithName("cronTask")

		taskLog.Info("Starting DevWorkspace backup job")
		if err := r.executeBackupSync(ctx, logger); err != nil {
			taskLog.Error(err, "Failed to execute backup job for DevWorkspaces")
		}
		taskLog.Info("DevWorkspace backup job finished")
	})
	if err != nil {
		log.Error(err, "Failed to add cronjob function")
		return
	}

	log.Info("Starting cron scheduler")
	r.cron.Start()
}

// stopCron stops the cron scheduler and removes all existing cronjob tasks.
func (r *BackupCronJobReconciler) stopCron(logger logr.Logger) {
	log := logger.WithName("backup cron")
	log.Info("Stopping cron scheduler")

	// remove existing cronjob tasks
	entries := r.cron.Entries()
	for _, entry := range entries {
		r.cron.Remove(entry.ID)
	}

	ctx := r.cron.Stop()
	ctx.Done()

	log.Info("Cron scheduler stopped")
}

// executeBackupSync executes the backup job for all DevWorkspaces in the cluster that
// have been stopped in the last N minutes.
func (r *BackupCronJobReconciler) executeBackupSync(ctx context.Context, logger logr.Logger) error {
	log := logger.WithName("executeBackupSync")
	log.Info("Executing backup sync for all DevWorkspaces")
	devWorkspaces := &dw.DevWorkspaceList{}
	err := r.List(ctx, devWorkspaces)
	if err != nil {
		log.Error(err, "Failed to list DevWorkspaces")
		return err
	}
	for _, dw := range devWorkspaces.Items {
		if !r.wasStoppedInTimeRange(&dw, 30, ctx, logger) {
			log.Info("Skipping backup for DevWorkspace that wasn't stopped recently", "namespace", dw.Namespace, "name", dw.Name)
			continue
		}
		dwID := dw.Status.DevWorkspaceId
		log.Info("Found DevWorkspace", "namespace", dw.Namespace, "devworkspace", dw.Name, "id", dwID)

		if err := r.createBackupJob(&dw, ctx, logger); err != nil {
			log.Error(err, "Failed to create backup Job for DevWorkspace", "id", dwID)
			continue
		}
		log.Info("Backup Job created for DevWorkspace", "id", dwID)

	}
	return nil
}

// wasStoppedInTimeRange checks if the DevWorkspace was stopped in the last N minutes.
func (r *BackupCronJobReconciler) wasStoppedInTimeRange(workspace *dw.DevWorkspace, timeRangeInMinute float64, ctx context.Context, logger logr.Logger) bool {
	log := logger.WithName("wasStoppedInTimeRange")
	if workspace.Status.Phase != dw.DevWorkspaceStatusStopped {
		return false
	}
	log.Info("DevWorkspace is currently stopped, checking if it was stopped recently", "namespace", workspace.Namespace, "name", workspace.Name)
	// Check if the workspace was stopped in the last N minutes
	if workspace.Status.Conditions != nil {
		lastTimeStopped := metav1.Time{}
		for _, condition := range workspace.Status.Conditions {
			if condition.Type == conditions.Started && condition.Status == corev1.ConditionFalse {
				lastTimeStopped = condition.LastTransitionTime
			}
		}
		// Calculate the time difference
		if !lastTimeStopped.IsZero() {
			timeDiff := metav1.Now().Sub(lastTimeStopped.Time)
			if timeDiff.Minutes() <= timeRangeInMinute {
				log.Info("DevWorkspace was stopped recently", "namespace", workspace.Namespace, "name", workspace.Name)
				return true
			}
		}
	}
	return false
}

// createBackupJob creates a Kubernetes Job to back up the workspace's PVC data.
func (r *BackupCronJobReconciler) createBackupJob(workspace *dw.DevWorkspace, ctx context.Context, logger logr.Logger) error {
	log := logger.WithName("createBackupJob")
	dwID := workspace.Status.DevWorkspaceId

	// Find a PVC with the name "claim-devworkspace"
	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, client.ObjectKey{Name: "claim-devworkspace", Namespace: workspace.Namespace}, pvc)
	if err != nil {
		log.Error(err, "Failed to get PVC for DevWorkspace", "id", dwID)
		return err
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "backup-job-",
			Namespace:    workspace.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "backup",
							Image:   defaultBackupImage,
							Command: []string{"/bin/sh", "-c"},
							Env: []corev1.EnvVar{
								{
									Name:  "DEVWORKSPACE_NAME",
									Value: workspace.Name,
								},
								{
									Name:  "DEVWORKSPACE_NAMESPACE",
									Value: workspace.Namespace,
								},
								{
									Name:  "WORKSPACE_ID",
									Value: dwID,
								},
								{
									Name:  "BACKUP_SOURCE_PATH",
									Value: "/workspace/" + dwID + "/" + constants.DefaultProjectsSourcesRoot,
								},
							},
							// TODO: Replace the following command with actual backup logic
							Args: []string{
								"echo \"Starting backup for workspace $WORKSPACE_ID\" && ls -l \"$BACKUP_SOURCE_PATH\" && sleep 1 && echo Backup completed for workspace " + dwID,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									MountPath: "/workspace",
									Name:      "workspace-data",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "workspace-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: pvc.Name,
								},
							},
						},
					},
				},
			},
		},
	}
	err = r.Create(ctx, job)
	if err != nil {
		log.Error(err, "Failed to create backup Job for DevWorkspace", "devworkspace", workspace.Name)
		return err
	}
	log.Info("Created backup Job for DevWorkspace", "jobName", job.Name, "devworkspace", workspace.Name)
	return nil
}
