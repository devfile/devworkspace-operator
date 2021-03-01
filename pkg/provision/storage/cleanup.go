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

package storage

import (
	"fmt"
	"path"
	"time"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/devfile/devworkspace-operator/controllers/workspace/provision"
	"github.com/devfile/devworkspace-operator/internal/images"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	pvcClaimMountPath = "/tmp/devworkspaces/"
	cleanupCommandFmt = "rm -rf %s"
)

var (
	cleanupJobCompletions      = int32(1)
	cleanupJobBackoffLimit     = int32(3)
	pvcCleanupPodMemoryLimit   = resource.MustParse(constants.PVCCleanupPodMemoryLimit)
	pvcCleanupPodMemoryRequest = resource.MustParse(constants.PVCCleanupPodMemoryRequest)
	pvcCleanupPodCPULimit      = resource.MustParse(constants.PVCCleanupPodCPULimit)
	pvcCleanupPodCPURequest    = resource.MustParse(constants.PVCCleanupPodCPURequest)
)

func runCommonPVCCleanupJob(workspace *dw.DevWorkspace, clusterAPI provision.ClusterAPI) error {
	PVCexists, err := commonPVCExists(workspace, clusterAPI)
	if err != nil {
		return err
	} else if !PVCexists {
		// Nothing to do; return nil and continue
		return nil
	}

	specJob, err := getSpecCommonPVCCleanupJob(workspace, clusterAPI)
	if err != nil {
		return err
	}
	clusterJob, err := getClusterCommonPVCCleanupJob(workspace, clusterAPI)
	if err != nil {
		return err
	}
	if clusterJob == nil {
		err := clusterAPI.Client.Create(clusterAPI.Ctx, specJob)
		if err != nil && !k8sErrors.IsAlreadyExists(err) {
			return err
		}
		return &NotReadyError{
			Message: "Created PVC cleanup job",
		}
	}
	if !equality.Semantic.DeepDerivative(specJob.Spec, clusterJob.Spec) {
		propagationPolicy := metav1.DeletePropagationBackground
		err := clusterAPI.Client.Delete(clusterAPI.Ctx, clusterJob, &client.DeleteOptions{PropagationPolicy: &propagationPolicy})
		if err != nil {
			return err
		}
		return &NotReadyError{
			Message: "Need to recreate PVC cleanup job",
		}
	}
	for _, condition := range clusterJob.Status.Conditions {
		if condition.Status != corev1.ConditionTrue {
			continue
		}
		switch condition.Type {
		case batchv1.JobComplete:
			return nil
		case batchv1.JobFailed:
			return &ProvisioningError{
				Message: fmt.Sprintf("DevWorkspace PVC cleanup job failed: see logs for job %q for details", clusterJob.Name),
			}
		}
	}
	// Requeue at least each 10 seconds to check if PVC is not removed by someone else
	return &NotReadyError{
		Message:      "Cleanup job is not in completed state",
		RequeueAfter: 10 * time.Second,
	}
}

func getSpecCommonPVCCleanupJob(workspace *dw.DevWorkspace, clusterAPI provision.ClusterAPI) (*batchv1.Job, error) {
	workspaceId := workspace.Status.WorkspaceId
	pvcName := config.ControllerCfg.GetWorkspacePVCName()
	jobLabels := map[string]string{
		constants.WorkspaceIDLabel: workspaceId,
	}
	if restrictedAccess, needsRestrictedAccess := workspace.Annotations[constants.WorkspaceRestrictedAccessAnnotation]; needsRestrictedAccess {
		jobLabels[constants.WorkspaceRestrictedAccessAnnotation] = restrictedAccess
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

	err := controllerutil.SetControllerReference(workspace, job, clusterAPI.Scheme)
	if err != nil {
		return nil, err
	}
	return job, nil
}

func getClusterCommonPVCCleanupJob(workspace *dw.DevWorkspace, clusterAPI provision.ClusterAPI) (*batchv1.Job, error) {
	namespacedName := types.NamespacedName{
		Name:      common.PVCCleanupJobName(workspace.Status.WorkspaceId),
		Namespace: workspace.Namespace,
	}
	clusterJob := &batchv1.Job{}

	err := clusterAPI.Client.Get(clusterAPI.Ctx, namespacedName, clusterJob)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return clusterJob, nil
}

func commonPVCExists(workspace *dw.DevWorkspace, clusterAPI provision.ClusterAPI) (bool, error) {
	namespacedName := types.NamespacedName{
		Name:      config.ControllerCfg.GetWorkspacePVCName(),
		Namespace: workspace.Namespace,
	}
	err := clusterAPI.Client.Get(clusterAPI.Ctx, namespacedName, &corev1.PersistentVolumeClaim{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return false, nil
		}

		return false, err
	}
	return true, nil
}
