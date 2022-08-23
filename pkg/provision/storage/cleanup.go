//
// Copyright (c) 2019-2022 Red Hat, Inc.
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

	"github.com/devfile/devworkspace-operator/pkg/library/status"
	nsconfig "github.com/devfile/devworkspace-operator/pkg/provision/config"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/devfile/devworkspace-operator/internal/images"
	"github.com/devfile/devworkspace-operator/pkg/common"
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

func runCommonPVCCleanupJob(workspace *common.DevWorkspaceWithConfig, clusterAPI sync.ClusterAPI) error {
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
	clusterObj, err := sync.SyncObjectWithCluster(specJob, clusterAPI)
	switch t := err.(type) {
	case nil:
		break
	case *sync.NotInSyncError:
		return &NotReadyError{Message: t.Error()}
	case *sync.UnrecoverableSyncError:
		return &ProvisioningError{Message: "Failed to sync cleanup job with cluster", Err: t.Cause}
	default:
		return err
	}

	clusterJob := clusterObj.(*batchv1.Job)
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

	msg, err := status.CheckPodsState(workspace.Status.DevWorkspaceId, clusterJob.Namespace, k8sclient.MatchingLabels{"job-name": common.PVCCleanupJobName(workspace.Status.DevWorkspaceId)}, clusterAPI, &workspace.Config)
	if err != nil {
		return &ProvisioningError{
			Err: err,
		}
	}

	if msg != "" {
		errMsg := fmt.Sprintf("DevWorkspace common PVC cleanup job failed: see logs for job %q for details. Additional information: %s", clusterJob.Name, msg)
		return &ProvisioningError{
			Message: errMsg,
		}
	}

	// Requeue at least each 10 seconds to check if PVC is not removed by someone else
	return &NotReadyError{
		Message:      "Cleanup job is not in completed state",
		RequeueAfter: 10 * time.Second,
	}
}

func getSpecCommonPVCCleanupJob(workspaceWithConfig *common.DevWorkspaceWithConfig, clusterAPI sync.ClusterAPI) (*batchv1.Job, error) {
	workspaceId := workspaceWithConfig.Status.DevWorkspaceId

	pvcName, err := checkForExistingCommonPVC(workspaceWithConfig.Namespace, clusterAPI)
	if err != nil {
		return nil, err
	}
	if pvcName == "" {
		pvcName = workspaceWithConfig.Config.Workspace.PVCName
	}

	jobLabels := map[string]string{
		constants.DevWorkspaceIDLabel:      workspaceId,
		constants.DevWorkspaceNameLabel:    workspaceWithConfig.Name,
		constants.DevWorkspaceCreatorLabel: workspaceWithConfig.Labels[constants.DevWorkspaceCreatorLabel],
	}
	if restrictedAccess, needsRestrictedAccess := workspaceWithConfig.Annotations[constants.DevWorkspaceRestrictedAccessAnnotation]; needsRestrictedAccess {
		jobLabels[constants.DevWorkspaceRestrictedAccessAnnotation] = restrictedAccess
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.PVCCleanupJobName(workspaceId),
			Namespace: workspaceWithConfig.Namespace,
			Labels:    jobLabels,
		},
		Spec: batchv1.JobSpec{
			Completions:  &cleanupJobCompletions,
			BackoffLimit: &cleanupJobBackoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: jobLabels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:   "Never",
					SecurityContext: wsprovision.GetDevWorkspaceSecurityContext(workspaceWithConfig.Config),
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

	podTolerations, nodeSelector, err := nsconfig.GetNamespacePodTolerationsAndNodeSelector(workspaceWithConfig.Namespace, clusterAPI)
	if err != nil {
		return nil, err
	}
	if podTolerations != nil && len(podTolerations) > 0 {
		job.Spec.Template.Spec.Tolerations = podTolerations
	}
	if nodeSelector != nil && len(nodeSelector) > 0 {
		job.Spec.Template.Spec.NodeSelector = nodeSelector
	}

	if err := controllerutil.SetControllerReference(workspaceWithConfig, job, clusterAPI.Scheme); err != nil {
		return nil, err
	}
	return job, nil
}

func commonPVCExists(workspaceWithConfig *common.DevWorkspaceWithConfig, clusterAPI sync.ClusterAPI) (bool, error) {
	namespacedName := types.NamespacedName{
		Name:      workspaceWithConfig.Config.Workspace.PVCName,
		Namespace: workspaceWithConfig.Namespace,
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
