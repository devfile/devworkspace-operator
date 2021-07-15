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

package provision

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/workspace/env"
	maputils "github.com/devfile/devworkspace-operator/internal/map"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var ContainerFailureStateReasons = []string{
	"CrashLoopBackOff",
	"ImagePullBackOff",
	"CreateContainerError",
	"RunContainerError",
}

type DeploymentProvisioningStatus struct {
	ProvisioningStatus
}

var deploymentDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(appsv1.Deployment{}, "TypeMeta", "ObjectMeta", "Status"),
	cmpopts.IgnoreFields(appsv1.DeploymentSpec{}, "RevisionHistoryLimit", "ProgressDeadlineSeconds"),
	cmpopts.IgnoreFields(corev1.PodSpec{}, "DNSPolicy", "SchedulerName", "DeprecatedServiceAccount"),
	cmpopts.IgnoreFields(corev1.Container{}, "TerminationMessagePath", "TerminationMessagePolicy", "ImagePullPolicy"),
	cmpopts.SortSlices(func(a, b corev1.Container) bool {
		return strings.Compare(a.Name, b.Name) > 0
	}),
	cmpopts.SortSlices(func(a, b corev1.Volume) bool {
		return strings.Compare(a.Name, b.Name) > 0
	}),
	cmpopts.SortSlices(func(a, b corev1.VolumeMount) bool {
		return strings.Compare(a.Name, b.Name) > 0
	}),
}

func SyncDeploymentToCluster(
	workspace *dw.DevWorkspace,
	podAdditions []v1alpha1.PodAdditions,
	saName string,
	clusterAPI ClusterAPI) DeploymentProvisioningStatus {

	automountPodAdditions, automountEnv, err := getAutoMountResources(workspace.Namespace, clusterAPI.Client)
	if err != nil {
		return DeploymentProvisioningStatus{
			ProvisioningStatus{Err: err},
		}
	}
	if err := checkAutoMountVolumesForCollision(podAdditions, automountPodAdditions); err != nil {
		return DeploymentProvisioningStatus{
			ProvisioningStatus{Err: err, FailStartup: true},
		}
	}
	podAdditions = append(podAdditions, automountPodAdditions...)

	var envFromSourceAdditions []corev1.EnvFromSource
	if automountEnv != nil {
		envFromSourceAdditions = append(envFromSourceAdditions, automountEnv...)
	}

	// [design] we have to pass components and routing pod additions separately because we need mountsources from each
	// component.
	specDeployment, err := getSpecDeployment(workspace, podAdditions, envFromSourceAdditions, saName, clusterAPI.Scheme)
	if err != nil {
		return DeploymentProvisioningStatus{
			ProvisioningStatus{
				Err:         err,
				FailStartup: true,
			},
		}
	}
	clusterDeployment, err := getClusterDeployment(specDeployment.Name, workspace.Namespace, clusterAPI.Client)
	if err != nil {
		return DeploymentProvisioningStatus{
			ProvisioningStatus{Err: err},
		}
	}

	if clusterDeployment == nil {
		clusterAPI.Logger.Info("Creating deployment...")
		if err := clusterAPI.Client.Create(context.TODO(), specDeployment); err != nil {
			return DeploymentProvisioningStatus{
				ProvisioningStatus{
					Err:         err,
					FailStartup: k8sErrors.IsInvalid(err),
				},
			}
		}
		return DeploymentProvisioningStatus{
			ProvisioningStatus{Requeue: true},
		}
	}

	if !cmp.Equal(specDeployment.Spec.Selector, clusterDeployment.Spec.Selector) {
		clusterAPI.Logger.Info("Deployment selector is different. Recreating deployment...")
		clusterDeployment.Spec = specDeployment.Spec
		err := clusterAPI.Client.Delete(context.TODO(), clusterDeployment)
		if err != nil {
			return DeploymentProvisioningStatus{ProvisioningStatus{Err: err}}
		}
		return DeploymentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Requeue: true},
		}
	}

	if !cmp.Equal(specDeployment, clusterDeployment, deploymentDiffOpts) {
		clusterAPI.Logger.Info("Updating deployment...")
		clusterDeployment.Spec = specDeployment.Spec
		err := clusterAPI.Client.Update(context.TODO(), clusterDeployment)
		if err != nil {
			if k8sErrors.IsConflict(err) {
				return DeploymentProvisioningStatus{ProvisioningStatus{Requeue: true}}
			} else if k8sErrors.IsInvalid(err) {
				return DeploymentProvisioningStatus{ProvisioningStatus{Err: err, FailStartup: true}}
			}
			return DeploymentProvisioningStatus{ProvisioningStatus{Err: err}}
		}
		return DeploymentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Requeue: true},
		}
	}

	deploymentReady := checkDeploymentStatus(clusterDeployment)
	if deploymentReady {
		return DeploymentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{
				Continue: true,
			},
		}
	}

	failureMsg, checkErr := checkFailedPods(workspace, clusterAPI)
	if checkErr != nil {
		return DeploymentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{
				Err: checkErr,
			},
		}
	}

	return DeploymentProvisioningStatus{
		ProvisioningStatus: ProvisioningStatus{
			FailStartup: failureMsg != "",
			Message:     failureMsg,
		},
	}
}

// DeleteWorkspaceDeployment deletes the deployment for the DevWorkspace
func DeleteWorkspaceDeployment(ctx context.Context, workspace *dw.DevWorkspace, client runtimeClient.Client) (wait bool, err error) {
	clusterDeployment, err := getClusterDeployment(common.DeploymentName(workspace.Status.DevWorkspaceId), workspace.Namespace, client)
	if err != nil {
		return false, err
	}
	if clusterDeployment == nil {
		return false, nil
	}
	err = client.Delete(ctx, clusterDeployment)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ScaleDeploymentToZero scales the cluster deployment to zero
func ScaleDeploymentToZero(workspace *dw.DevWorkspace, client runtimeClient.Client) error {
	patch := []byte(`{"spec":{"replicas": 0}}`)
	err := client.Patch(context.Background(), &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: workspace.Namespace,
			Name:      common.DeploymentName(workspace.Status.DevWorkspaceId),
		},
	}, runtimeClient.RawPatch(types.StrategicMergePatchType, patch))

	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}

	return nil
}

func GetDevWorkspaceSecurityContext() *corev1.PodSecurityContext {
	if !infrastructure.IsOpenShift() {
		uID := int64(1234)
		rootGID := int64(0)
		nonRoot := true
		return &corev1.PodSecurityContext{
			RunAsUser:    &uID,
			RunAsGroup:   &rootGID,
			RunAsNonRoot: &nonRoot,
		}
	}
	return &corev1.PodSecurityContext{}
}

func checkDeploymentStatus(deployment *appsv1.Deployment) (ready bool) {
	return deployment.Status.ReadyReplicas > 0
}

func getSpecDeployment(
	workspace *dw.DevWorkspace,
	podAdditionsList []v1alpha1.PodAdditions,
	envFromSourceAdditions []corev1.EnvFromSource,
	saName string,
	scheme *runtime.Scheme) (*appsv1.Deployment, error) {
	replicas := int32(1)
	terminationGracePeriod := int64(1)

	podAdditions, err := mergePodAdditions(podAdditionsList)
	if err != nil {
		return nil, err
	}

	creator := workspace.Labels[constants.DevWorkspaceCreatorLabel]
	var envVars []corev1.EnvVar
	envVars = append(envVars, env.CommonEnvironmentVariables(workspace.Name, workspace.Status.DevWorkspaceId, workspace.Namespace, creator)...)
	for idx := range podAdditions.Containers {
		podAdditions.Containers[idx].Env = append(podAdditions.Containers[idx].Env, envVars...)
		podAdditions.Containers[idx].VolumeMounts = append(podAdditions.Containers[idx].VolumeMounts, podAdditions.VolumeMounts...)
		podAdditions.Containers[idx].EnvFrom = append(podAdditions.Containers[idx].EnvFrom, envFromSourceAdditions...)
	}
	for idx := range podAdditions.InitContainers {
		podAdditions.InitContainers[idx].Env = append(podAdditions.InitContainers[idx].Env, envVars...)
		podAdditions.InitContainers[idx].VolumeMounts = append(podAdditions.InitContainers[idx].VolumeMounts, podAdditions.VolumeMounts...)
		podAdditions.InitContainers[idx].EnvFrom = append(podAdditions.InitContainers[idx].EnvFrom, envFromSourceAdditions...)
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.DeploymentName(workspace.Status.DevWorkspaceId),
			Namespace: workspace.Namespace,
			Labels: map[string]string{
				constants.DevWorkspaceIDLabel:   workspace.Status.DevWorkspaceId,
				constants.DevWorkspaceNameLabel: workspace.Name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					constants.DevWorkspaceIDLabel: workspace.Status.DevWorkspaceId,
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
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
					SecurityContext:               GetDevWorkspaceSecurityContext(),
					ServiceAccountName:            saName,
					AutomountServiceAccountToken:  nil,
				},
			},
		},
	}

	if needsPVCWorkaround(podAdditions) {
		// Kubernetes creates directories in a PVC to support subpaths such that only the leaf directory has g+rwx permissions.
		// This means that mounting the subpath e.g. <workspace-id>/plugins will result in the <workspace-id> directory being
		// created with 755 permissions, requiring the root UID to remove it.
		// To avoid this issue, we need to ensure that the first volumeMount encountered is for the <workspace-id> subpath.
		if len(deployment.Spec.Template.Spec.InitContainers) > 0 {
			volumeMounts := deployment.Spec.Template.Spec.InitContainers[0].VolumeMounts
			volumeMounts = append([]corev1.VolumeMount{getWorkspaceSubpathVolumeMount(workspace.Status.DevWorkspaceId)}, volumeMounts...)
			deployment.Spec.Template.Spec.InitContainers[0].VolumeMounts = volumeMounts
		} else {
			volumeMounts := deployment.Spec.Template.Spec.Containers[0].VolumeMounts
			volumeMounts = append([]corev1.VolumeMount{getWorkspaceSubpathVolumeMount(workspace.Status.DevWorkspaceId)}, volumeMounts...)
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

	err = controllerutil.SetControllerReference(workspace, deployment, scheme)
	if err != nil {
		return nil, err
	}

	return deployment, nil
}

func getClusterDeployment(name string, namespace string, client runtimeClient.Client) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{}
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	err := client.Get(context.TODO(), namespacedName, deployment)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return deployment, nil
}

func getPods(workspace *dw.DevWorkspace, client runtimeClient.Client) (*corev1.PodList, error) {
	pods := &corev1.PodList{}
	if err := client.List(context.TODO(), pods, k8sclient.InNamespace(workspace.Namespace), k8sclient.MatchingLabels{
		constants.DevWorkspaceIDLabel: workspace.Status.DevWorkspaceId,
	}); err != nil {
		return nil, err
	}
	return pods, nil
}

// checkFailedPods check if related pods has unrecoverable states: CrashLoopBackOffReason, ImagePullErr
// Returns optional message with detected unrecoverable state details
//         error is any happens during check
func checkFailedPods(workspace *dw.DevWorkspace,
	clusterAPI ClusterAPI) (stateMsg string, checkFailure error) {
	podList, err := getPods(workspace, clusterAPI.Client)
	if err != nil {
		return "", err
	}

	for _, pod := range podList.Items {
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if !checkContainerStatusForFailure(&containerStatus) {
				return fmt.Sprintf("Container %s has state %s", containerStatus.Name, containerStatus.State.Waiting.Reason), nil
			}
		}
		for _, initContainerStatus := range pod.Status.InitContainerStatuses {
			if !checkContainerStatusForFailure(&initContainerStatus) {
				return fmt.Sprintf("Init Container %s has state %s", initContainerStatus.Name, initContainerStatus.State.Waiting.Reason), nil
			}
		}
	}
	return "", nil
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

func getWorkspaceSubpathVolumeMount(workspaceId string) corev1.VolumeMount {
	volumeName := config.ControllerCfg.GetWorkspacePVCName()

	workspaceVolumeMount := corev1.VolumeMount{
		Name:      volumeName,
		MountPath: "/tmp/workspace-storage",
		SubPath:   workspaceId,
	}

	return workspaceVolumeMount
}

func needsPVCWorkaround(podAdditions *v1alpha1.PodAdditions) bool {
	commonPVCName := config.ControllerCfg.GetWorkspacePVCName()
	for _, vol := range podAdditions.Volumes {
		if vol.Name == commonPVCName {
			return true
		}
	}
	return false
}

func checkContainerStatusForFailure(containerStatus *corev1.ContainerStatus) (ok bool) {
	if containerStatus.State.Waiting != nil {
		for _, failureReason := range ContainerFailureStateReasons {
			if containerStatus.State.Waiting.Reason == failureReason {
				return false
			}
		}
	}
	return true
}
