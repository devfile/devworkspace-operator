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
	"fmt"
	"path"
	"time"

	devworkspace "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/controllers/workspace/provision"
	"github.com/devfile/devworkspace-operator/internal/images"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/config"
	storagelib "github.com/devfile/devworkspace-operator/pkg/library/storage"

	"github.com/go-logr/logr"
	coputil "github.com/redhat-cop/operator-utils/pkg/util"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	pvcClaimMountPath   = "/tmp/devworkspaces/"
	cleanupCommandFmt   = "rm -rf %s"
	pvcCleanupFinalizer = "storage.controller.devfile.io"
)

var (
	cleanupJobCompletions      = int32(1)
	cleanupJobBackoffLimit     = int32(3)
	pvcCleanupPodMemoryLimit   = resource.MustParse(config.PVCCleanupPodMemoryLimit)
	pvcCleanupPodMemoryRequest = resource.MustParse(config.PVCCleanupPodMemoryRequest)
	pvcCleanupPodCPULimit      = resource.MustParse(config.PVCCleanupPodCPULimit)
	pvcCleanupPodCPURequest    = resource.MustParse(config.PVCCleanupPodCPURequest)
)

func (r *DevWorkspaceReconciler) finalize(ctx context.Context, log logr.Logger, workspace *devworkspace.DevWorkspace) (reconcile.Result, error) {
	if !coputil.HasFinalizer(workspace, pvcCleanupFinalizer) {
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
		//Namespace is terminating, it's redundant to clean PVC files since it's going to be removed
		log.Info("Namespace is terminating; clearing storage finalizer")
		coputil.RemoveFinalizer(workspace, pvcCleanupFinalizer)
		return reconcile.Result{}, r.Update(ctx, workspace)
	}

	pvcExists, err := r.pvcExists(ctx, workspace)
	if err != nil {
		return reconcile.Result{}, err
	} else if !pvcExists {
		//PVC does not exist. nothing to clean up
		log.Info("PVC does not exit; clearing storage finalizer")
		coputil.RemoveFinalizer(workspace, pvcCleanupFinalizer)
		// job will be clean up by k8s garbage collector
		return reconcile.Result{}, r.Update(ctx, workspace)
	}

	specJob, err := r.getSpecCleanupJob(workspace)
	if err != nil {
		return reconcile.Result{}, err
	}

	clusterJob, err := r.getClusterCleanupJob(ctx, workspace)
	if err != nil {
		return reconcile.Result{}, err
	}
	if clusterJob == nil {
		err := r.Create(ctx, specJob)
		if err != nil && !k8sErrors.IsAlreadyExists(err) {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}
	if !equality.Semantic.DeepDerivative(specJob.Spec, clusterJob.Spec) {
		propagationPolicy := metav1.DeletePropagationBackground
		err := r.Delete(ctx, clusterJob, &client.DeleteOptions{PropagationPolicy: &propagationPolicy})
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}
	for _, condition := range clusterJob.Status.Conditions {
		if condition.Status != corev1.ConditionTrue {
			continue
		}
		switch condition.Type {
		case batchv1.JobComplete:
			log.Info("PVC clean up job successful; clearing finalizer")
			coputil.RemoveFinalizer(workspace, pvcCleanupFinalizer)
			return reconcile.Result{}, r.Update(ctx, workspace)
		case batchv1.JobFailed:
			log.Error(fmt.Errorf("PVC clean up job failed: message: %q", condition.Message),
				"Failed to clean PVC on workspace deletion")
			failedStatus := &currentStatus{
				Conditions: map[devworkspace.WorkspaceConditionType]string{
					"Error": fmt.Sprintf("Failed to clean PVC on deletion. See logs for job %q for details", clusterJob.Name),
				},
				Phase: "Error",
			}
			return r.updateWorkspaceStatus(workspace, r.Log, failedStatus, reconcile.Result{}, nil)
		}
	}
	// Requeue at least each 10 seconds to check if PVC is not removed by someone else
	return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *DevWorkspaceReconciler) getSpecCleanupJob(workspace *devworkspace.DevWorkspace) (*batchv1.Job, error) {
	workspaceId := workspace.Status.WorkspaceId
	pvcName := config.ControllerCfg.GetWorkspacePVCName()
	jobLabels := map[string]string{
		config.WorkspaceIDLabel: workspaceId,
	}
	if restrictedAccess, needsRestrictedAccess := workspace.Annotations[config.WorkspaceRestrictedAccessAnnotation]; needsRestrictedAccess {
		jobLabels[config.WorkspaceRestrictedAccessAnnotation] = restrictedAccess
	}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.PVCCleanupJobName(workspaceId),
			Namespace: workspace.Namespace,
			Labels:    jobLabels,
		},
		Spec: batchv1.JobSpec{
			Completions:  &cleanupJobCompletions,
			BackoffLimit: &cleanupJobBackoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:   "Never",
					SecurityContext: provision.GetDevWorkspaceSecurityContext(),
					Volumes: []corev1.Volume{
						{
							Name: pvcName,
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: pvcName,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:    common.PVCCleanupJobName(workspaceId),
							Image:   images.GetPVCCleanupJobImage(),
							Command: []string{"/bin/sh"},
							Args: []string{
								"-c",
								fmt.Sprintf(cleanupCommandFmt, path.Join(pvcClaimMountPath, workspaceId)),
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: pvcCleanupPodMemoryRequest,
									corev1.ResourceCPU:    pvcCleanupPodCPURequest,
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: pvcCleanupPodMemoryLimit,
									corev1.ResourceCPU:    pvcCleanupPodCPULimit,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      pvcName,
									MountPath: pvcClaimMountPath,
								},
							},
						},
					},
				},
			},
		},
	}

	err := controllerutil.SetControllerReference(workspace, job, r.Scheme)
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (r *DevWorkspaceReconciler) getClusterCleanupJob(ctx context.Context, workspace *devworkspace.DevWorkspace) (*batchv1.Job, error) {
	namespacedName := types.NamespacedName{
		Name:      common.PVCCleanupJobName(workspace.Status.WorkspaceId),
		Namespace: workspace.Namespace,
	}
	clusterJob := &batchv1.Job{}

	err := r.Get(ctx, namespacedName, clusterJob)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return clusterJob, nil
}

func isFinalizerNecessary(workspace *devworkspace.DevWorkspace) bool {
	return storagelib.NeedsStorage(workspace.Spec.Template)
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

func (r *DevWorkspaceReconciler) pvcExists(ctx context.Context, workspace *devworkspace.DevWorkspace) (bool, error) {
	namespacedName := types.NamespacedName{
		Name:      config.ControllerCfg.GetWorkspacePVCName(),
		Namespace: workspace.Namespace,
	}
	err := r.Get(ctx, namespacedName, &corev1.PersistentVolumeClaim{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return false, nil
		}

		return false, err
	}
	return true, nil
}
