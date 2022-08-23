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

	"github.com/devfile/devworkspace-operator/pkg/library/status"
	nsconfig "github.com/devfile/devworkspace-operator/pkg/provision/config"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	maputils "github.com/devfile/devworkspace-operator/internal/map"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
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

type DeploymentProvisioningStatus struct {
	ProvisioningStatus
}

func SyncDeploymentToCluster(
	workspace *dw.DevWorkspace,
	podAdditions []v1alpha1.PodAdditions,
	saName string,
	clusterAPI sync.ClusterAPI, config *controllerv1alpha1.OperatorConfiguration) DeploymentProvisioningStatus {

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
	specDeployment, err := getSpecDeployment(workspace, podAdditions, saName, podTolerations, nodeSelector, clusterAPI.Scheme, *config)
	if err != nil {
		return DeploymentProvisioningStatus{
			ProvisioningStatus{
				Err:         err,
				FailStartup: true,
			},
		}
	}
	if len(specDeployment.Spec.Template.Spec.Containers) == 0 {
		// DevWorkspace defines no container components, cannot create a deployment
		return DeploymentProvisioningStatus{ProvisioningStatus{Continue: true}}
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

	deploymentReady := status.CheckDeploymentStatus(clusterDeployment)
	if deploymentReady {
		return DeploymentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{
				Continue: true,
			},
		}
	}

	deploymentHealthy, deploymentErrMsg := status.CheckDeploymentConditions(clusterDeployment)
	if !deploymentHealthy {
		return DeploymentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{
				FailStartup: true,
				Message:     deploymentErrMsg,
			},
		}
	}

	failureMsg, checkErr := status.CheckPodsState(workspace.Status.DevWorkspaceId, workspace.Namespace, k8sclient.MatchingLabels{
		constants.DevWorkspaceIDLabel: workspace.Status.DevWorkspaceId,
	}, clusterAPI, config)
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

func GetDevWorkspaceSecurityContext(config controllerv1alpha1.OperatorConfiguration) *corev1.PodSecurityContext {
	if infrastructure.IsOpenShift() {
		return &corev1.PodSecurityContext{}
	}
	return config.Workspace.PodSecurityContext
}

func getSpecDeployment(
	workspace *dw.DevWorkspace,
	podAdditionsList []v1alpha1.PodAdditions,
	saName string,
	podTolerations []corev1.Toleration,
	nodeSelector map[string]string,
	scheme *runtime.Scheme, config controllerv1alpha1.OperatorConfiguration) (*appsv1.Deployment, error) {
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
					SecurityContext:               GetDevWorkspaceSecurityContext(config),
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
	if workspace.Spec.Template.Attributes.Exists(constants.RuntimeClassNameAttribute) {
		runtimeClassName := workspace.Spec.Template.Attributes.GetString(constants.RuntimeClassNameAttribute, nil)
		if runtimeClassName != "" {
			deployment.Spec.Template.Spec.RuntimeClassName = &runtimeClassName
		}
	}

	if needPVC, pvcName := needsPVCWorkaround(podAdditions, config); needPVC {
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

	err = controllerutil.SetControllerReference(workspace, deployment, scheme)
	if err != nil {
		return nil, err
	}

	return deployment, nil
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

func needsPVCWorkaround(podAdditions *v1alpha1.PodAdditions, config controllerv1alpha1.OperatorConfiguration) (needs bool, pvcName string) {
	commonPVCName := config.Workspace.PVCName
	for _, vol := range podAdditions.Volumes {
		if vol.Name == commonPVCName {
			return true, commonPVCName
		}
		if vol.Name == constants.CheCommonPVCName {
			return true, constants.CheCommonPVCName
		}
	}
	return false, ""
}

func getAdditionalAnnotations(workspace *dw.DevWorkspace) (map[string]string, error) {
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
