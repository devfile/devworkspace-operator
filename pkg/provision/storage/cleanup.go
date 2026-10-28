//
// Copyright (c) 2019-2026 Red Hat, Inc.
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
	"slices"
	"time"

	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	"github.com/devfile/devworkspace-operator/pkg/library/status"
	nsconfig "github.com/devfile/devworkspace-operator/pkg/provision/config"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/devfile/devworkspace-operator/internal/images"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
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
	if err != nil {
		return dwerrors.WrapSyncError(err)
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
			return &dwerrors.FailError{
				Message: fmt.Sprintf("DevWorkspace PVC cleanup job failed: see logs for job %q for details", clusterJob.Name),
			}
		}
	}

	jobLabels := k8sclient.MatchingLabels{"job-name": common.PVCCleanupJobName(workspace.Status.DevWorkspaceId)}
	msg, err := status.CheckPodsState(workspace.Status.DevWorkspaceId, clusterJob.Namespace, jobLabels, workspace.Config.Workspace.IgnoredUnrecoverableEvents, clusterAPI)
	if err != nil {
		return &dwerrors.FailError{
			Message: "Error while checking cleanup job pods state",
			Err:     err,
		}
	}

	if msg != "" {
		errMsg := fmt.Sprintf("DevWorkspace common PVC cleanup job failed: see logs for job %q for details. Additional information: %s", clusterJob.Name, msg)
		return &dwerrors.FailError{
			Message: errMsg,
		}
	}

	// Requeue at least each 10 seconds to check if PVC is not removed by someone else
	return &dwerrors.RetryError{
		Message:      "Cleanup job is not in completed state",
		RequeueAfter: 10 * time.Second,
	}
}

func getSpecCommonPVCCleanupJob(workspace *common.DevWorkspaceWithConfig, clusterAPI sync.ClusterAPI) (*batchv1.Job, error) {
	workspaceId := workspace.Status.DevWorkspaceId

	_, pvcName, err := checkForAlternatePVC(workspace.Namespace, clusterAPI)
	if err != nil {
		return nil, err
	}
	if pvcName == "" {
		pvcName = workspace.Config.Workspace.PVCName
	}

	targetNode, err := FindNodeForPVC(clusterAPI.Ctx, clusterAPI.Client, workspace.Namespace, pvcName)
	if err != nil {
		clusterAPI.Logger.Error(err, "Error getting target node for cleanup job")
	} else if targetNode == "" {
		clusterAPI.Logger.Info("No target node for cleanup job, NodeAffinity will not be defined")
	}

	jobLabels := map[string]string{
		constants.DevWorkspaceIDLabel:      workspaceId,
		constants.DevWorkspaceNameLabel:    workspace.Name,
		constants.DevWorkspaceCreatorLabel: workspace.Labels[constants.DevWorkspaceCreatorLabel],
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
				ObjectMeta: metav1.ObjectMeta{
					Labels: jobLabels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:   "Never",
					SecurityContext: workspace.Config.Workspace.PodSecurityContext,
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
					Affinity: &corev1.Affinity{},
				},
			},
		},
	}

	if targetNode != "" {
		job.Spec.Template.Spec.Affinity = NodeAffinityForHostname(targetNode)
	}

	podTolerations, nodeSelector, err := nsconfig.GetNamespacePodTolerationsAndNodeSelector(workspace.Namespace, clusterAPI)
	if err != nil {
		return nil, err
	}
	if podTolerations != nil && len(podTolerations) > 0 {
		job.Spec.Template.Spec.Tolerations = podTolerations
	}
	if nodeSelector != nil && len(nodeSelector) > 0 {
		job.Spec.Template.Spec.NodeSelector = nodeSelector
	}

	if err := controllerutil.SetControllerReference(workspace.DevWorkspace, job, clusterAPI.Scheme); err != nil {
		return nil, err
	}
	return job, nil
}

func commonPVCExists(workspace *common.DevWorkspaceWithConfig, clusterAPI sync.ClusterAPI) (bool, error) {
	namespacedName := types.NamespacedName{
		Name:      workspace.Config.Workspace.PVCName,
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

// deleteCommonPVC deletes the shared PVC used for workspace storage in the workspace's
// namespace, coordinating with cleanup jobs and leftover pods that still reference it.
// Deleting the PVC while such references exist leaves it stuck in a Terminating state
// (the pvc-protection finalizer holds it as long as any pod uses it), which blocks new
// workspaces in the namespace from starting.
func deleteCommonPVC(workspace *common.DevWorkspaceWithConfig, clusterAPI sync.ClusterAPI) error {
	namespace := workspace.Namespace

	_, pvcName, err := checkForAlternatePVC(namespace, clusterAPI)
	if err != nil {
		return err
	}
	if pvcName == "" {
		pvcName = workspace.Config.Workspace.PVCName
	}

	labelSelector, err := labels.Parse(constants.DevWorkspaceIDLabel)
	if err != nil {
		return err
	}

	// Resolve leftover pods that reference the PVC.
	podList := &corev1.PodList{}
	if err := clusterAPI.Client.List(clusterAPI.Ctx, podList, &k8sclient.ListOptions{
		Namespace:     namespace,
		LabelSelector: labelSelector,
	}); err != nil {
		return err
	}
	for _, pod := range podList.Items {
		if !isPVCMounted(pod.Spec, pvcName) {
			continue
		}
		// Job-owned pods are not removed when the job finishes, so a finished
		// cleanup job can leave its pod holding the PVC in a Terminating state;
		// delete the pod so the pvc-protection finalizer can release the PVC.
		jobName := getOwningJobName(&pod)
		if jobName != "" {
			job := &batchv1.Job{}
			if err := clusterAPI.Client.Get(clusterAPI.Ctx, types.NamespacedName{Name: jobName, Namespace: pod.Namespace}, job); err != nil {
				if !k8sErrors.IsNotFound(err) {
					return err
				}
				// The owning job is gone, so the pod will be garbage-collected via its owner reference.
			} else if isJobFinished(job) {
				if err := clusterAPI.Client.Delete(clusterAPI.Ctx, &pod); err != nil && !k8sErrors.IsNotFound(err) {
					return err
				}
				return &dwerrors.RetryError{
					Message:      fmt.Sprintf("Deleted pod %s of finished job %s using PVC %s", pod.Name, jobName, pvcName),
					RequeueAfter: 3 * time.Second,
				}
			}
		}
		if len(pod.OwnerReferences) == 0 {
			if err := clusterAPI.Client.Delete(clusterAPI.Ctx, &pod); err != nil && !k8sErrors.IsNotFound(err) {
				return err
			}
			return &dwerrors.RetryError{
				Message:      fmt.Sprintf("Deleted orphaned pod %s using PVC %s", pod.Name, pvcName),
				RequeueAfter: 3 * time.Second,
			}
		}

		waitMsg := fmt.Sprintf("Waiting for pod %s using PVC %s to be deleted before removing the PVC", pod.Name, pvcName)
		if jobName != "" {
			waitMsg = fmt.Sprintf("Waiting for pod %s of job %s using PVC %s to be deleted before removing the PVC", pod.Name, jobName, pvcName)
		}
		return &dwerrors.RetryError{
			Message:      waitMsg,
			RequeueAfter: 10 * time.Second,
		}
	}

	pvc := &corev1.PersistentVolumeClaim{}
	namespacedName := types.NamespacedName{Name: pvcName, Namespace: namespace}
	err = clusterAPI.Client.Get(clusterAPI.Ctx, namespacedName, pvc)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	err = clusterAPI.Client.Delete(clusterAPI.Ctx, pvc)
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}
	return nil
}

func isPVCMounted(spec corev1.PodSpec, pvcName string) bool {
	return slices.ContainsFunc(
		spec.Volumes,
		func(volume corev1.Volume) bool {
			return volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvcName
		},
	)
}

// getOwningJobName returns the name of the Job that controls the pod, or an
// empty string if the pod is not controlled by a Job.
func getOwningJobName(pod *corev1.Pod) string {
	if ref := metav1.GetControllerOf(pod); ref != nil && ref.Kind == "Job" {
		return ref.Name
	}
	return ""
}

func isJobFinished(job *batchv1.Job) bool {
	for _, condition := range job.Status.Conditions {
		if condition.Status != corev1.ConditionTrue {
			continue
		}
		switch condition.Type {
		case batchv1.JobComplete, batchv1.JobFailed:
			return true
		}
	}
	return false
}
