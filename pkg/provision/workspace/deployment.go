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

package workspace

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	"github.com/devfile/devworkspace-operator/pkg/library/overrides"
	"github.com/devfile/devworkspace-operator/pkg/library/status"
	nsconfig "github.com/devfile/devworkspace-operator/pkg/provision/config"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	maputils "github.com/devfile/devworkspace-operator/internal/map"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func SyncDeploymentToCluster(
	workspace *common.DevWorkspaceWithConfig,
	podAdditions []v1alpha1.PodAdditions,
	saName string,
	clusterAPI sync.ClusterAPI) error {

	podTolerations, nodeSelector, err := nsconfig.GetNamespacePodTolerationsAndNodeSelector(workspace.Namespace, clusterAPI)
	if err != nil {
		return &dwerrors.FailError{Message: "Failed to read pod tolerations and node selector from namespace", Err: err}
	}

	// [design] we have to pass components and routing pod additions separately because we need mountsources from each
	// component.
	specDeployment, err := getSpecDeployment(workspace, podAdditions, saName, podTolerations, nodeSelector, clusterAPI.Scheme)
	if err != nil {
		return &dwerrors.FailError{Message: "Error while creating workspace deployment", Err: err}
	}
	if len(specDeployment.Spec.Template.Spec.Containers) == 0 {
		// DevWorkspace defines no container components, cannot create a deployment
		return nil
	}

	clusterObj, err := sync.SyncObjectWithCluster(specDeployment, clusterAPI)
	if err != nil {
		return dwerrors.WrapSyncError(err)
	}

	clusterDeployment := clusterObj.(*appsv1.Deployment)
	deploymentReady := status.CheckDeploymentStatus(clusterDeployment, workspace)
	if !deploymentReady {
		deploymentHealthy, deploymentErrMsg := status.CheckDeploymentConditions(clusterDeployment)
		if !deploymentHealthy {
			return &dwerrors.FailError{Message: deploymentErrMsg}
		}

		workspaceIDLabel := k8sclient.MatchingLabels{constants.DevWorkspaceIDLabel: workspace.Status.DevWorkspaceId}
		ignoredEvents := workspace.Config.Workspace.IgnoredUnrecoverableEvents
		failureMsg, checkErr := status.CheckPodsState(workspace.Status.DevWorkspaceId, workspace.Namespace, workspaceIDLabel, ignoredEvents, clusterAPI)
		if checkErr != nil {
			return err
		}
		if failureMsg != "" {
			return &dwerrors.FailError{Message: failureMsg}
		}

		return &dwerrors.RetryError{Message: "Deployment is not ready"}
	}

	return nil
}

// DeleteWorkspaceDeployment deletes the deployment for the DevWorkspace
func DeleteWorkspaceDeployment(ctx context.Context, workspace *common.DevWorkspaceWithConfig, client k8sclient.Client) (wait bool, err error) {
	err = client.Delete(ctx, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: workspace.Namespace,
			Name:      common.DeploymentName(workspace.Status.DevWorkspaceId),
		},
	})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ScaleDeploymentToZero scales the cluster deployment to zero
func ScaleDeploymentToZero(ctx context.Context, workspace *common.DevWorkspaceWithConfig, client k8sclient.Client) error {
	patch := []byte(`{"spec":{"replicas": 0}}`)
	err := client.Patch(ctx, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: workspace.Namespace,
			Name:      common.DeploymentName(workspace.Status.DevWorkspaceId),
		},
	}, k8sclient.RawPatch(types.StrategicMergePatchType, patch))

	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}

	return nil
}

func getSpecDeployment(
	workspace *common.DevWorkspaceWithConfig,
	podAdditionsList []v1alpha1.PodAdditions,
	saName string,
	podTolerations []corev1.Toleration,
	nodeSelector map[string]string,
	scheme *runtime.Scheme) (*appsv1.Deployment, error) {
	replicas := int32(1)
	terminationGracePeriod := int64(10)

	podAdditions, err := mergePodAdditions(podAdditionsList)
	if err != nil {
		return nil, err
	}

	for idx := range podAdditions.Containers {
		podAdditions.Containers[idx].VolumeMounts = append(podAdditions.Containers[idx].VolumeMounts, podAdditions.VolumeMounts...)
	}
	for idx := range podAdditions.InitContainers {
		podAdditions.InitContainers[idx].VolumeMounts = append(podAdditions.InitContainers[idx].VolumeMounts, podAdditions.VolumeMounts...)
	}

	labels := map[string]string{}
	labels[constants.DevWorkspaceIDLabel] = workspace.Status.DevWorkspaceId
	labels[constants.DevWorkspaceNameLabel] = workspace.Name

	annotations, err := getAdditionalAnnotations(workspace)
	if err != nil {
		return nil, err
	}

	deploymentStrategy := appsv1.DeploymentStrategy{
		Type: workspace.Config.Workspace.DeploymentStrategy,
	}
	if workspace.Config.Workspace.DeploymentStrategy == appsv1.RollingUpdateDeploymentStrategyType {
		deploymentStrategy.RollingUpdate = &appsv1.RollingUpdateDeployment{
			MaxUnavailable: &constants.RollingUpdateMaxUnavailable,
			MaxSurge:       &constants.RollingUpdateMaximumSurge,
		}
	}
	progressDeadlineSeconds, err := getProgressDeadlineSeconds(workspace.Config)
	if err != nil {
		return nil, err
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        common.DeploymentName(workspace.Status.DevWorkspaceId),
			Namespace:   workspace.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					constants.DevWorkspaceIDLabel: workspace.Status.DevWorkspaceId,
				},
			},
			Strategy:                deploymentStrategy,
			ProgressDeadlineSeconds: &progressDeadlineSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      workspace.Status.DevWorkspaceId,
					Namespace: workspace.Namespace,
					Labels: map[string]string{
						constants.DevWorkspaceIDLabel:   workspace.Status.DevWorkspaceId,
						constants.DevWorkspaceNameLabel: workspace.Name,
					},
				},
				Spec: corev1.PodSpec{
					InitContainers:                podAdditions.InitContainers,
					Containers:                    podAdditions.Containers,
					ImagePullSecrets:              podAdditions.PullSecrets,
					Volumes:                       podAdditions.Volumes,
					RestartPolicy:                 "Always",
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					SchedulerName:                 workspace.Config.Workspace.SchedulerName,
					SecurityContext:               workspace.Config.Workspace.PodSecurityContext,
					ServiceAccountName:            saName,
					AutomountServiceAccountToken:  nil,
				},
			},
		},
	}

	if overrides.NeedsPodOverrides(workspace) {
		patchedDeployment, err := overrides.ApplyPodOverrides(workspace, deployment)
		if err != nil {
			return nil, err
		}
		deployment = patchedDeployment
	}

	if len(podTolerations) > 0 {
		deployment.Spec.Template.Spec.Tolerations = podTolerations
	}
	if len(nodeSelector) > 0 {
		deployment.Spec.Template.Spec.NodeSelector = nodeSelector
	}
	if workspace.Spec.Template.Attributes.Exists(constants.RuntimeClassNameAttribute) {
		runtimeClassName := workspace.Spec.Template.Attributes.GetString(constants.RuntimeClassNameAttribute, nil)
		if runtimeClassName != "" {
			deployment.Spec.Template.Spec.RuntimeClassName = &runtimeClassName
		}
	}

	if needPVC, pvcName := needsPVCWorkaround(podAdditions, workspace.Config.Workspace.PVCName); needPVC {
		// Kubernetes creates directories in a PVC to support subpaths such that only the leaf directory has g+rwx permissions.
		// This means that mounting the subpath e.g. <workspace-id>/plugins will result in the <workspace-id> directory being
		// created with 755 permissions, requiring the root UID to remove it.
		// To avoid this issue, we need to ensure that the first volumeMount encountered is for the <workspace-id> subpath.
		if len(deployment.Spec.Template.Spec.InitContainers) > 0 {
			volumeMounts := deployment.Spec.Template.Spec.InitContainers[0].VolumeMounts
			volumeMounts = append([]corev1.VolumeMount{getWorkspaceSubpathVolumeMount(workspace.Status.DevWorkspaceId, pvcName)}, volumeMounts...)
			deployment.Spec.Template.Spec.InitContainers[0].VolumeMounts = volumeMounts
		} else {
			volumeMounts := deployment.Spec.Template.Spec.Containers[0].VolumeMounts
			volumeMounts = append([]corev1.VolumeMount{getWorkspaceSubpathVolumeMount(workspace.Status.DevWorkspaceId, pvcName)}, volumeMounts...)
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts
		}
	}

	workspaceCreator, present := workspace.Labels[constants.DevWorkspaceCreatorLabel]
	if present {
		deployment.Labels[constants.DevWorkspaceCreatorLabel] = workspaceCreator
		deployment.Spec.Template.Labels[constants.DevWorkspaceCreatorLabel] = workspaceCreator
	} else {
		return nil, errors.New("workspace must have creator specified to be run. Recreate it to fix an issue")
	}

	restrictedAccess, present := workspace.Annotations[constants.DevWorkspaceRestrictedAccessAnnotation]
	if present {
		deployment.Annotations = maputils.Append(deployment.Annotations, constants.DevWorkspaceRestrictedAccessAnnotation, restrictedAccess)
		deployment.Spec.Template.Annotations = maputils.Append(deployment.Spec.Template.Annotations, constants.DevWorkspaceRestrictedAccessAnnotation, restrictedAccess)
	}

	err = controllerutil.SetControllerReference(workspace.DevWorkspace, deployment, scheme)
	if err != nil {
		return nil, err
	}

	return deployment, nil
}

// Returns the ProgressDeadlineSeconds to use for workspace deployments as an int32.
// The ProgressDeadLineSeconds returned is the same length of time as DWOC's workspace.ProgressTimeout.
// Returns an error if ProgressTimeout could not be properly parsed,
// or if the ProgressTimeout would exceed the maximum capacity of an int32
func getProgressDeadlineSeconds(config *v1alpha1.OperatorConfiguration) (int32, error) {
	timeout, err := time.ParseDuration(config.Workspace.ProgressTimeout)
	if err != nil {
		return 0, fmt.Errorf("invalid duration specified for workspace progress timeout: %w", err)
	}
	// Prevent overflow
	if timeout.Seconds() > math.MaxInt32 {
		return 0, fmt.Errorf("duration specified for workspace progress timeout is too long: %s", config.Workspace.ProgressTimeout)
	}
	return int32(timeout.Seconds()), nil
}

func mergePodAdditions(toMerge []v1alpha1.PodAdditions) (*v1alpha1.PodAdditions, error) {
	podAdditions := &v1alpha1.PodAdditions{}

	// "Set"s to store k8s object names and detect duplicates
	containerNames := map[string]bool{}
	initContainerNames := map[string]bool{}
	volumeNames := map[string]bool{}
	volumeMountNames := map[string]bool{}
	pullSecretNames := map[string]bool{}
	for _, additions := range toMerge {
		for annotKey, annotVal := range additions.Annotations {
			podAdditions.Annotations[annotKey] = annotVal
		}
		for labelKey, labelVal := range additions.Labels {
			podAdditions.Labels[labelKey] = labelVal
		}
		for _, container := range additions.Containers {
			if containerNames[container.Name] {
				return nil, fmt.Errorf("duplicate containers in the workspace definition: %s", container.Name)
			}
			containerNames[container.Name] = true
			podAdditions.Containers = append(podAdditions.Containers, container)
		}

		for _, container := range additions.InitContainers {
			if initContainerNames[container.Name] {
				return nil, fmt.Errorf("duplicate init containers in the workspace definition: %s", container.Name)
			}
			initContainerNames[container.Name] = true
			podAdditions.InitContainers = append(podAdditions.InitContainers, container)
		}

		for _, volume := range additions.Volumes {
			if volumeNames[volume.Name] {
				return nil, fmt.Errorf("duplicate volumes in the workspace definition: %s", volume.Name)
			}
			volumeNames[volume.Name] = true
			podAdditions.Volumes = append(podAdditions.Volumes, volume)
		}

		for _, volumeMount := range additions.VolumeMounts {
			if volumeMountNames[volumeMount.Name] {
				return nil, fmt.Errorf("duplicated volumeMounts in workspace definition: %s", volumeMount.Name)
			}
			volumeMountNames[volumeMount.Name] = true
			podAdditions.VolumeMounts = append(podAdditions.VolumeMounts, volumeMount)
		}

		for _, pullSecret := range additions.PullSecrets {
			if pullSecretNames[pullSecret.Name] {
				continue
			}
			pullSecretNames[pullSecret.Name] = true
			podAdditions.PullSecrets = append(podAdditions.PullSecrets, pullSecret)
		}
	}
	return podAdditions, nil
}

func getWorkspaceSubpathVolumeMount(workspaceId, pvcName string) corev1.VolumeMount {
	workspaceVolumeMount := corev1.VolumeMount{
		Name:      pvcName,
		MountPath: "/tmp/workspace-storage",
		SubPath:   workspaceId,
	}

	return workspaceVolumeMount
}

func needsPVCWorkaround(podAdditions *v1alpha1.PodAdditions, pvcName string) (needs bool, workaroundPvcName string) {
	for _, vol := range podAdditions.Volumes {
		if vol.Name == pvcName {
			return true, pvcName
		}
		if vol.Name == constants.CheCommonPVCName {
			return true, constants.CheCommonPVCName
		}
	}
	return false, ""
}

func getAdditionalAnnotations(workspace *common.DevWorkspaceWithConfig) (map[string]string, error) {
	annotations := map[string]string{}

	for _, component := range workspace.Spec.Template.Components {
		if component.Container == nil || component.Container.Annotation == nil || component.Container.Annotation.Deployment == nil {
			continue
		}
		for k, v := range component.Container.Annotation.Deployment {
			if currValue, exists := annotations[k]; exists && v != currValue {
				return nil, fmt.Errorf("conflicting annotations found on container components for key %s", k)
			}
			annotations[k] = v
		}
	}

	return annotations, nil
}
