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
	"reflect"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/internal/images"
	"github.com/devfile/devworkspace-operator/pkg/conditions"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/devfile/devworkspace-operator/pkg/library/storage"
	"github.com/go-logr/logr"
	"github.com/robfig/cron/v3"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// BackupCronJobReconciler reconciles `BackupCronJob` configuration for the purpose of backing up workspace PVCs.
type BackupCronJobReconciler struct {
	client.Client
	NonCachingClient client.Client
	Log              logr.Logger
	Scheme           *runtime.Scheme

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

	return !reflect.DeepEqual(oldBackup, newBackup)
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
		Watches(
			&controllerv1alpha1.DevWorkspaceOperatorConfig{},
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
			builder.WithPredicates(configPredicate),
		).
		Watches(
			&batchv1.Job{},
			r.getBackupJobEventHandler(),
			builder.WithPredicates(r.getBackupJobPredicate()),
		).
		Complete(r)
}

// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts;,verbs=get;list;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;create;update;patch;delete;watch
// +kubebuilder:rbac:groups=controller.devfile.io,resources=devworkspaceoperatorconfigs,verbs=get;list;update;patch;watch
// +kubebuilder:rbac:groups=workspace.devfile.io,resources=devworkspaces,verbs=get;list;update;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=builds,verbs=get
// +kubebuilder:rbac:groups="",resources=builds/details,verbs=update
// +kubebuilder:rbac:groups="",resources=imagestreams,verbs=create
// +kubebuilder:rbac:groups="",resources=imagestreams/layers,verbs=get;update
// +kubebuilder:rbac:groups=build.openshift.io,resources=builds,verbs=get
// +kubebuilder:rbac:groups=build.openshift.io,resources=builds/details,verbs=update
// +kubebuilder:rbac:groups=image.openshift.io,resources=imagestreams,verbs=get;list;create;update;patch;delete
// +kubebuilder:rbac:groups=image.openshift.io,resources=imagestreams/layers,verbs=get;update

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

	r.startCron(ctx, dwOperatorConfig, log)

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
func (r *BackupCronJobReconciler) startCron(ctx context.Context, dwOperatorConfig *controllerv1alpha1.DevWorkspaceOperatorConfig, log logr.Logger) {
	log.Info("Starting backup cron scheduler")

	// remove existing cronjob tasks
	// we cannot update the existing tasks, so we need to remove them and add new ones
	entries := r.cron.Entries()
	for _, entry := range entries {
		log.Info("Removing existing cronjob task", "entryID", entry.ID)
		r.cron.Remove(entry.ID)
	}

	// add cronjob task
	backUpConfig := dwOperatorConfig.Config.Workspace.BackupCronJob
	log.Info("Adding cronjob task", "schedule", backUpConfig.Schedule)
	_, err := r.cron.AddFunc(backUpConfig.Schedule, func() {
		log.Info("Starting DevWorkspace backup job")
		if err := r.executeBackupSync(ctx, dwOperatorConfig, log); err != nil {
			log.Error(err, "Failed to execute backup job for DevWorkspaces")
		}
		log.Info("DevWorkspace backup job finished")
	})
	if err != nil {
		log.Error(err, "Failed to add cronjob function")
		return
	}

	log.Info("Starting cron scheduler")
	r.cron.Start()
}

// stopCron stops the cron scheduler and removes all existing cronjob tasks.
func (r *BackupCronJobReconciler) stopCron(log logr.Logger) {
	log.Info("Stopping cron scheduler")

	// remove existing cronjob tasks
	entries := r.cron.Entries()
	for _, entry := range entries {
		r.cron.Remove(entry.ID)
	}

	ctx := r.cron.Stop()
	<-ctx.Done()

	log.Info("Cron scheduler stopped")
}

// executeBackupSync executes the backup job for all DevWorkspaces in the cluster that
// have been stopped since their last backup.
func (r *BackupCronJobReconciler) executeBackupSync(ctx context.Context, dwOperatorConfig *controllerv1alpha1.DevWorkspaceOperatorConfig, log logr.Logger) error {
	log.Info("Executing backup sync for all DevWorkspaces")

	devWorkspaces := &dw.DevWorkspaceList{}
	err := r.List(ctx, devWorkspaces)
	if err != nil {
		log.Error(err, "Failed to list DevWorkspaces")
		return err
	}
	var lastBackupTime *metav1.Time
	if dwOperatorConfig.Status != nil && dwOperatorConfig.Status.LastBackupTime != nil {
		lastBackupTime = dwOperatorConfig.Status.LastBackupTime
	}
	for _, dw := range devWorkspaces.Items {
		if !r.wasStoppedSinceLastBackup(&dw, lastBackupTime, log) {
			log.Info("Skipping backup for DevWorkspace that wasn't stopped recently", "namespace", dw.Namespace, "name", dw.Name)
			continue
		}
		dwID := dw.Status.DevWorkspaceId
		log.Info("Found DevWorkspace", "namespace", dw.Namespace, "devworkspace", dw.Name, "id", dwID)

		err = r.ensureJobRunnerRBAC(ctx, &dw)
		if err != nil {
			log.Error(err, "Failed to ensure Job runner RBAC for DevWorkspace", "id", dwID)
			continue
		}

		if err = r.createBackupJob(&dw, ctx, dwOperatorConfig, log); err != nil {
			log.Error(err, "Failed to create backup Job for DevWorkspace", "id", dwID)
			continue
		}
		log.Info("Backup Job created for DevWorkspace", "id", dwID)

	}
	origConfig := client.MergeFrom(dwOperatorConfig.DeepCopy())
	if dwOperatorConfig.Status == nil {
		dwOperatorConfig.Status = &controllerv1alpha1.OperatorConfigurationStatus{}
	}
	dwOperatorConfig.Status.LastBackupTime = &metav1.Time{Time: metav1.Now().Time}

	err = r.Status().Patch(ctx, dwOperatorConfig, origConfig)
	if err != nil {
		log.Error(err, "Failed to update DevWorkspaceOperatorConfig status with last backup time")
		return err
	}
	return nil
}

// wasStoppedSinceLastBackup checks if the DevWorkspace was stopped since its last backup.
// It reads the last backup time from the DevWorkspace annotation, or falls back to the
// provided globalLastBackupTime if the annotation doesn't exist.
func (r *BackupCronJobReconciler) wasStoppedSinceLastBackup(
	devWorkspace *dw.DevWorkspace,
	globalLastBackupTime *metav1.Time,
	log logr.Logger,
) bool {
	if devWorkspace.Status.Phase != dw.DevWorkspaceStatusStopped {
		return false
	}
	log.Info("DevWorkspace is currently stopped, checking if it was stopped since last backup", "namespace", devWorkspace.Namespace, "name", devWorkspace.Name)

	var lastBackupFinishedAt *metav1.Time
	var lastBackupSuccessful bool

	// Get the last backup time and success status from the workspace annotations
	if devWorkspace.Annotations != nil {
		if lastBackupFinishedAtStr, ok := devWorkspace.Annotations[constants.DevWorkspaceLastBackupFinishedAtAnnotation]; ok {
			parsedTime, err := time.Parse(time.RFC3339Nano, lastBackupFinishedAtStr)
			if err != nil {
				log.Error(err, "Failed to parse last backup time annotation, treating as no previous backup", "value", lastBackupFinishedAtStr)
			} else {
				lastBackupFinishedAt = &metav1.Time{Time: parsedTime}
			}
		}

		lastBackupSuccessful = devWorkspace.Annotations[constants.DevWorkspaceLastBackupSuccessfulAnnotation] == "true"
	}

	if lastBackupFinishedAt == nil {
		// Fall back to globalLastBackupTime if annotation doesn't exist
		lastBackupFinishedAt = globalLastBackupTime
		lastBackupSuccessful = true
	}

	if lastBackupFinishedAt == nil {
		return true
	}

	if !lastBackupSuccessful {
		return true
	}

	// Check if the workspace was stopped since the last successful backup
	if devWorkspace.Status.Conditions != nil {
		lastTimeStopped := metav1.Time{}
		for _, condition := range devWorkspace.Status.Conditions {
			if condition.Type == conditions.Started && condition.Status == corev1.ConditionFalse {
				lastTimeStopped = condition.LastTransitionTime
			}
		}

		if !lastTimeStopped.IsZero() {
			if lastTimeStopped.Time.After(lastBackupFinishedAt.Time) {
				log.Info("DevWorkspace was stopped since last successful backup", "namespace", devWorkspace.Namespace, "name", devWorkspace.Name)
				return true
			}
		}
	}

	return false
}

// createBackupJob creates a Kubernetes Job to back up the workspace's PVC data.
func (r *BackupCronJobReconciler) createBackupJob(
	workspace *dw.DevWorkspace,
	ctx context.Context,
	dwOperatorConfig *controllerv1alpha1.DevWorkspaceOperatorConfig,
	log logr.Logger,
) error {
	dwID := workspace.Status.DevWorkspaceId
	backUpConfig := dwOperatorConfig.Config.Workspace.BackupCronJob

	registryAuthSecret, err := r.handleRegistryAuthSecret(ctx, workspace, dwOperatorConfig, log)
	if err != nil {
		log.Error(err, "Failed to handle registry auth secret for DevWorkspace", "devworkspace", workspace.Name)
		return err
	}

	// Find a PVC with used by the workspace
	pvcName, workspacePath, err := storage.GetWorkspacePVCInfo(ctx, workspace, dwOperatorConfig.Config, r.Client, log)
	if err != nil {
		log.Error(err, "Failed to get workspace PVC name", "devworkspace", workspace.Name)
		return err
	}
	if pvcName == "" {
		log.Error(err, "No PVC found for DevWorkspace", "id", dwID)
		return err
	}

	pvc := &corev1.PersistentVolumeClaim{}
	err = r.Get(ctx, client.ObjectKey{Name: pvcName, Namespace: workspace.Namespace}, pvc)
	if err != nil {
		log.Error(err, "Failed to get PVC for DevWorkspace", "id", dwID)
		return err
	}
	orasExtraArgs := ""
	if backUpConfig.OrasConfig != nil {
		orasExtraArgs = backUpConfig.OrasConfig.ExtraArgs
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: constants.DevWorkspaceBackupJobNamePrefix,
			Namespace:    workspace.Namespace,
			Labels: map[string]string{
				constants.DevWorkspaceIDLabel:        dwID,
				constants.DevWorkspaceNameLabel:      workspace.Name,
				constants.DevWorkspaceBackupJobLabel: "true",
			},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: ptr.To[int32](120),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"io.kubernetes.cri-o.Devices": "/dev/fuse",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: JobRunnerSAName + "-" + workspace.Status.DevWorkspaceId,
					RestartPolicy:      corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name: "backup-workspace",
							Env: []corev1.EnvVar{
								{Name: "DEVWORKSPACE_NAME", Value: workspace.Name},
								{Name: "DEVWORKSPACE_NAMESPACE", Value: workspace.Namespace},
								{Name: "WORKSPACE_ID", Value: dwID},
								{
									Name:  "BACKUP_SOURCE_PATH",
									Value: "/workspace/" + workspacePath,
								},
								{Name: "DEVWORKSPACE_BACKUP_REGISTRY", Value: backUpConfig.Registry.Path},
								{Name: "ORAS_EXTRA_ARGS", Value: orasExtraArgs},
							},
							Image:           images.GetProjectBackupImage(),
							ImagePullPolicy: "Always",
							Args: []string{
								"/workspace-recovery.sh",
								"--backup",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									MountPath: "/workspace",
									Name:      "workspace-data",
								},
								{
									MountPath: "/var/lib/containers",
									Name:      "build-storage",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To[bool](false),
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
						{
							Name: "build-storage",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}
	if registryAuthSecret != nil {
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "registry-auth-secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: registryAuthSecret.Name,
				},
			},
		})
		job.Spec.Template.Spec.Containers[0].VolumeMounts = append(job.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "registry-auth-secret",
			MountPath: "/tmp/.docker",
			ReadOnly:  true,
		})
		job.Spec.Template.Spec.Containers[0].Env = append(job.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  "REGISTRY_AUTH_FILE",
			Value: "/tmp/.docker/.dockerconfigjson",
		})

	}
	if err := controllerutil.SetControllerReference(workspace, job, r.Scheme); err != nil {
		return err
	}
	err = r.Create(ctx, job)
	if err != nil {
		log.Error(err, "Failed to create backup Job for DevWorkspace", "devworkspace", workspace.Name)
		return err
	}
	log.Info("Created backup Job for DevWorkspace", "jobName", job.Name, "devworkspace", workspace.Name)
	return nil
}

func (r *BackupCronJobReconciler) handleRegistryAuthSecret(ctx context.Context, workspace *dw.DevWorkspace,
	dwOperatorConfig *controllerv1alpha1.DevWorkspaceOperatorConfig, log logr.Logger,
) (*corev1.Secret, error) {
	secretName := dwOperatorConfig.Config.Workspace.BackupCronJob.Registry.AuthSecret
	if secretName == "" {
		// No auth secret configured - anonymous access to registry
		return nil, nil
	}

	// First check the workspace namespace for the secret
	registryAuthSecret := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{
		Name:      secretName,
		Namespace: workspace.Namespace}, registryAuthSecret)
	if err == nil {
		log.Info("Successfully retrieved registry auth secret for backup from workspace namespace", "secretName", secretName)
		return registryAuthSecret, nil
	}
	if client.IgnoreNotFound(err) != nil {
		return nil, err
	}

	log.Info("Registry auth secret not found in workspace namespace, checking operator namespace", "secretName", secretName)

	// If the secret is not found in the workspace namespace, check the operator namespace as fallback
	err = r.Get(ctx, client.ObjectKey{
		Name:      secretName,
		Namespace: dwOperatorConfig.Namespace}, registryAuthSecret)
	if err != nil {
		log.Error(err, "Failed to get registry auth secret for backup job", "secretName", secretName)
		return nil, err
	}
	log.Info("Successfully retrieved registry auth secret for backup job", "secretName", secretName)
	return r.copySecret(ctx, workspace, registryAuthSecret, log)
}

// copySecret copies the given secret from the operator namespace to the workspace namespace.
func (r *BackupCronJobReconciler) copySecret(ctx context.Context, workspace *dw.DevWorkspace, sourceSecret *corev1.Secret, log logr.Logger) (namespaceSecret *corev1.Secret, err error) {
	existingNamespaceSecret := &corev1.Secret{}
	err = r.Get(ctx, client.ObjectKey{
		Name:      constants.DevWorkspaceBackupAuthSecretName,
		Namespace: workspace.Namespace}, existingNamespaceSecret)
	if client.IgnoreNotFound(err) != nil {
		log.Error(err, "Failed to check for existing registry auth secret in workspace namespace", "namespace", workspace.Namespace)
		return nil, err
	}
	if err == nil {
		err = r.Delete(ctx, existingNamespaceSecret)
		if err != nil {
			return nil, err
		}
	}
	namespaceSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.DevWorkspaceBackupAuthSecretName,
			Namespace: workspace.Namespace,
			Labels: map[string]string{
				constants.DevWorkspaceIDLabel:          workspace.Status.DevWorkspaceId,
				constants.DevWorkspaceWatchSecretLabel: "true",
			},
		},
		Data: sourceSecret.Data,
		Type: sourceSecret.Type,
	}
	if err := controllerutil.SetControllerReference(workspace, namespaceSecret, r.Scheme); err != nil {
		return nil, err
	}
	err = r.Create(ctx, namespaceSecret)
	if err == nil {
		log.Info("Successfully created secret", "name", namespaceSecret.Name, "namespace", workspace.Namespace)
	}
	return namespaceSecret, err
}
