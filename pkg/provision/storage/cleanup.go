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

package storage

import (
	"fmt"
	"path"
	"time"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/devfile/devworkspace-operator/internal/images"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	wsprovision "github.com/devfile/devworkspace-operator/pkg/provision/workspace"
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

func runCommonPVCCleanupJob(workspace *dw.DevWorkspace, clusterAPI wsprovision.ClusterAPI) error {
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

func getSpecCommonPVCCleanupJob(workspace *dw.DevWorkspace, clusterAPI wsprovision.ClusterAPI) (*batchv1.Job, error) {
	workspaceId := workspace.Status.DevWorkspaceId
	pvcName := config.Workspace.PVCName
	jobLabels := map[string]string{
		constants.DevWorkspaceIDLabel: workspaceId,
	}
	if restrictedAccess, needsRestrictedAccess := workspace.Annotations[constants.DevWorkspaceRestrictedAccessAnnotation]; needsRestrictedAccess {
		jobLabels[constants.DevWorkspaceRestrictedAccessAnnotation] = restrictedAccess
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
					SecurityContext: wsprovision.GetDevWorkspaceSecurityContext(),
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

func getClusterCommonPVCCleanupJob(workspace *dw.DevWorkspace, clusterAPI wsprovision.ClusterAPI) (*batchv1.Job, error) {
	namespacedName := types.NamespacedName{
		Name:      common.PVCCleanupJobName(workspace.Status.DevWorkspaceId),
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

func commonPVCExists(workspace *dw.DevWorkspace, clusterAPI wsprovision.ClusterAPI) (bool, error) {
	namespacedName := types.NamespacedName{
		Name:      config.Workspace.PVCName,
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
