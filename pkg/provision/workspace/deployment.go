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

package workspace

import (
	"context"
	"errors"
	"fmt"
	"strings"

	nsconfig "github.com/devfile/devworkspace-operator/pkg/provision/config"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	"k8s.io/apimachinery/pkg/fields"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	maputils "github.com/devfile/devworkspace-operator/internal/map"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"

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

var containerFailureStateReasons = []string{
	"CrashLoopBackOff",
	"ImagePullBackOff",
	"CreateContainerError",
	"RunContainerError",
}

var unrecoverablePodEventReasons = []string{
	"FailedPostStartHook",
	"FailedMount",
	"FailedScheduling",
	"FailedCreate",
	"ReplicaSetCreateError",
}

var unrecoverableDeploymentConditionReasons = []string{
	"FailedCreate",
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
	clusterAPI sync.ClusterAPI) DeploymentProvisioningStatus {

	podTolerations, nodeSelector, err := nsconfig.GetNamespacePodTolerationsAndNodeSelector(workspace.Namespace, clusterAPI)
	if err != nil {
		return DeploymentProvisioningStatus{
			ProvisioningStatus{
				Message:     "failed to read pod tolerations and node selector from namespace",
				Err:         err,
				FailStartup: true,
			},
		}
	}

	// [design] we have to pass components and routing pod additions separately because we need mountsources from each
	// component.
	specDeployment, err := getSpecDeployment(workspace, podAdditions, saName, podTolerations, nodeSelector, clusterAPI.Scheme)
	if err != nil {
		return DeploymentProvisioningStatus{
			ProvisioningStatus{
				Err:         err,
				FailStartup: true,
			},
		}
	}

	clusterObj, err := sync.SyncObjectWithCluster(specDeployment, clusterAPI)
	switch t := err.(type) {
	case nil:
		break
	case *sync.NotInSyncError:
		return DeploymentProvisioningStatus{
			ProvisioningStatus{Requeue: true},
		}
	case *sync.UnrecoverableSyncError:
		return DeploymentProvisioningStatus{
			ProvisioningStatus{FailStartup: true, Err: t.Cause},
		}
	default:
		return DeploymentProvisioningStatus{ProvisioningStatus{Err: err}}
	}
	clusterDeployment := clusterObj.(*appsv1.Deployment)

	deploymentReady := checkDeploymentStatus(clusterDeployment)
	if deploymentReady {
		return DeploymentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{
				Continue: true,
			},
		}
	}

	deploymentHealthy, deploymentErrMsg := checkDeploymentConditions(clusterDeployment)
	if !deploymentHealthy {
		return DeploymentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{
				FailStartup: true,
				Message:     deploymentErrMsg,
			},
		}
	}

	failureMsg, checkErr := checkPodsState(workspace, clusterAPI)
	if checkErr != nil {
		return DeploymentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{
				Err: checkErr,
			},
		}
	}
	if failureMsg != "" {
		return DeploymentProvisioningStatus{
			ProvisioningStatus{
				FailStartup: true,
				Message:     failureMsg,
			},
		}
	}

	return DeploymentProvisioningStatus{}
}

// DeleteWorkspaceDeployment deletes the deployment for the DevWorkspace
func DeleteWorkspaceDeployment(ctx context.Context, workspace *dw.DevWorkspace, client runtimeClient.Client) (wait bool, err error) {
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
func ScaleDeploymentToZero(ctx context.Context, workspace *dw.DevWorkspace, client runtimeClient.Client) error {
	patch := []byte(`{"spec":{"replicas": 0}}`)
	err := client.Patch(ctx, &appsv1.Deployment{
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
		fsGroup := int64(1234)
		rootGID := int64(0)
		nonRoot := true
		return &corev1.PodSecurityContext{
			RunAsUser:    &uID,
			RunAsGroup:   &rootGID,
			RunAsNonRoot: &nonRoot,
			FSGroup:      &fsGroup,
		}
	}
	return &corev1.PodSecurityContext{}
}

func checkDeploymentStatus(deployment *appsv1.Deployment) (ready bool) {
	return deployment.Status.ReadyReplicas > 0
}

func checkDeploymentConditions(deployment *appsv1.Deployment) (healthy bool, errorMsg string) {
	conditions := deployment.Status.Conditions
	for _, condition := range conditions {
		for _, unrecoverableReason := range unrecoverableDeploymentConditionReasons {
			if condition.Reason == unrecoverableReason {
				return false, fmt.Sprintf("Detected unrecoverable deployment condition: %s %s", condition.Reason, condition.Message)
			}
		}
	}
	return true, ""
}

func getSpecDeployment(
	workspace *dw.DevWorkspace,
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

	if podTolerations != nil && len(podTolerations) > 0 {
		deployment.Spec.Template.Spec.Tolerations = podTolerations
	}
	if nodeSelector != nil && len(nodeSelector) > 0 {
		deployment.Spec.Template.Spec.NodeSelector = nodeSelector
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

func getPods(workspace *dw.DevWorkspace, client runtimeClient.Client) (*corev1.PodList, error) {
	pods := &corev1.PodList{}
	if err := client.List(context.TODO(), pods, k8sclient.InNamespace(workspace.Namespace), k8sclient.MatchingLabels{
		constants.DevWorkspaceIDLabel: workspace.Status.DevWorkspaceId,
	}); err != nil {
		return nil, err
	}
	return pods, nil
}

// checkPodsState checks if workspace-related pods are in an unrecoverable state. A pod is considered to be unrecoverable
// if it has a container with one of the containerStateFailureReasons states, or if an unrecoverable event (with reason
// matching unrecoverablePodEventReasons) has the pod as the involved object.
// Returns optional message with detected unrecoverable state details
//         error if any happens during check
func checkPodsState(workspace *dw.DevWorkspace,
	clusterAPI sync.ClusterAPI) (stateMsg string, checkFailure error) {
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
		if msg, err := checkPodEvents(&pod, clusterAPI); err != nil || msg != "" {
			return msg, err
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
	volumeName := config.Workspace.PVCName

	workspaceVolumeMount := corev1.VolumeMount{
		Name:      volumeName,
		MountPath: "/tmp/workspace-storage",
		SubPath:   workspaceId,
	}

	return workspaceVolumeMount
}

func needsPVCWorkaround(podAdditions *v1alpha1.PodAdditions) bool {
	commonPVCName := config.Workspace.PVCName
	for _, vol := range podAdditions.Volumes {
		if vol.Name == commonPVCName {
			return true
		}
	}
	return false
}

func checkPodEvents(pod *corev1.Pod, clusterAPI sync.ClusterAPI) (msg string, err error) {
	evs := &corev1.EventList{}
	selector, err := fields.ParseSelector(fmt.Sprintf("involvedObject.name=%s", pod.Name))
	if err != nil {
		return "", fmt.Errorf("failed to parse field selector: %s", err)
	}
	if err := clusterAPI.Client.List(clusterAPI.Ctx, evs, k8sclient.InNamespace(pod.Namespace), k8sclient.MatchingFieldsSelector{Selector: selector}); err != nil {
		return "", fmt.Errorf("failed to list events in namespace %s: %w", pod.Namespace, err)
	}
	for _, ev := range evs.Items {
		if ev.InvolvedObject.Kind != "Pod" {
			continue
		}
		for _, fatalEv := range unrecoverablePodEventReasons {
			if ev.Reason == fatalEv && !checkIfUnrecoverableEventIgnored(ev.Reason) {
				return fmt.Sprintf("Detected unrecoverable event %s: %s", ev.Reason, ev.Message), nil
			}
		}
	}
	return "", nil
}

func checkContainerStatusForFailure(containerStatus *corev1.ContainerStatus) (ok bool) {
	if containerStatus.State.Waiting != nil {
		for _, failureReason := range containerFailureStateReasons {
			if containerStatus.State.Waiting.Reason == failureReason {
				return checkIfUnrecoverableEventIgnored(containerStatus.State.Waiting.Reason)
			}
		}
	}
	return true
}

func checkIfUnrecoverableEventIgnored(reason string) (ignored bool) {
	for _, ignoredReason := range config.Workspace.IgnoredUnrecoverableEvents {
		if ignoredReason == reason {
			return true
		}
	}
	return false
}
