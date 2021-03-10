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

	maputils "github.com/devfile/devworkspace-operator/internal/map"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"

	"github.com/devfile/devworkspace-operator/pkg/common"

	devworkspace "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/workspace/env"
	"github.com/devfile/devworkspace-operator/pkg/config"
)

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
}

func SyncDeploymentToCluster(
	workspace *devworkspace.DevWorkspace,
	podAdditions []v1alpha1.PodAdditions,
	saName string,
	clusterAPI ClusterAPI) DeploymentProvisioningStatus {

	// [design] we have to pass components and routing pod additions separately because we need mountsources from each
	// component.
	specDeployment, err := getSpecDeployment(workspace, podAdditions, saName, clusterAPI.Scheme)
	if err != nil {
		return DeploymentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{
				Err:         err,
				FailStartup: true,
			},
		}
	}

	clusterDeployment, err := getClusterDeployment(specDeployment.Name, workspace.Namespace, clusterAPI.Client)
	if err != nil {
		return DeploymentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err},
		}
	}

	if clusterDeployment == nil {
		clusterAPI.Logger.Info("Creating deployment...")
		err := clusterAPI.Client.Create(context.TODO(), specDeployment)
		return DeploymentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{
				Requeue: true,
				Err:     err,
			},
		}
	}

	if !cmp.Equal(specDeployment, clusterDeployment, deploymentDiffOpts) {
		clusterAPI.Logger.Info("Updating deployment...")
		clusterDeployment.Spec = specDeployment.Spec
		err := clusterAPI.Client.Update(context.TODO(), clusterDeployment)
		if err != nil {
			if k8sErrors.IsConflict(err) {
				return DeploymentProvisioningStatus{ProvisioningStatus: ProvisioningStatus{Requeue: true}}
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

	return DeploymentProvisioningStatus{}
}

// DeleteWorkspaceDeployment deletes the deployment for the DevWorkspace
func DeleteWorkspaceDeployment(ctx context.Context, workspace *devworkspace.DevWorkspace, client runtimeClient.Client) (wait bool, err error) {
	clusterDeployment, err := getClusterDeployment(common.DeploymentName(workspace.Status.WorkspaceId), workspace.Namespace, client)
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
	workspace *devworkspace.DevWorkspace,
	podAdditionsList []v1alpha1.PodAdditions,
	saName string,
	scheme *runtime.Scheme) (*appsv1.Deployment, error) {
	replicas := int32(1)
	terminationGracePeriod := int64(1)

	podAdditions, err := mergePodAdditions(podAdditionsList)
	if err != nil {
		return nil, err
	}

	creator := workspace.Labels[constants.WorkspaceCreatorLabel]
	commonEnv := env.CommonEnvironmentVariables(workspace.Name, workspace.Status.WorkspaceId, workspace.Namespace, creator)
	for idx := range podAdditions.Containers {
		podAdditions.Containers[idx].Env = append(podAdditions.Containers[idx].Env, commonEnv...)
		podAdditions.Containers[idx].VolumeMounts = append(podAdditions.Containers[idx].VolumeMounts, podAdditions.VolumeMounts...)
	}
	for idx := range podAdditions.InitContainers {
		podAdditions.InitContainers[idx].Env = append(podAdditions.InitContainers[idx].Env, commonEnv...)
		podAdditions.InitContainers[idx].VolumeMounts = append(podAdditions.InitContainers[idx].VolumeMounts, podAdditions.VolumeMounts...)
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.DeploymentName(workspace.Status.WorkspaceId),
			Namespace: workspace.Namespace,
			Labels: map[string]string{
				constants.WorkspaceIDLabel: workspace.Status.WorkspaceId,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					constants.WorkspaceIDLabel:   workspace.Status.WorkspaceId,
					constants.WorkspaceNameLabel: workspace.Name,
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      workspace.Status.WorkspaceId,
					Namespace: workspace.Namespace,
					Labels: map[string]string{
						constants.WorkspaceIDLabel:   workspace.Status.WorkspaceId,
						constants.WorkspaceNameLabel: workspace.Name,
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
			volumeMounts = append([]corev1.VolumeMount{getWorkspaceSubpathVolumeMount(workspace.Status.WorkspaceId)}, volumeMounts...)
			deployment.Spec.Template.Spec.InitContainers[0].VolumeMounts = volumeMounts
		} else {
			volumeMounts := deployment.Spec.Template.Spec.Containers[0].VolumeMounts
			volumeMounts = append([]corev1.VolumeMount{getWorkspaceSubpathVolumeMount(workspace.Status.WorkspaceId)}, volumeMounts...)
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts
		}
	}

	workspaceCreator, present := workspace.Labels[constants.WorkspaceCreatorLabel]
	if present {
		deployment.Labels[constants.WorkspaceCreatorLabel] = workspaceCreator
		deployment.Spec.Template.Labels[constants.WorkspaceCreatorLabel] = workspaceCreator
	} else {
		return nil, errors.New("workspace must have creator specified to be run. Recreate it to fix an issue")
	}

	restrictedAccess, present := workspace.Annotations[constants.WorkspaceRestrictedAccessAnnotation]
	if present {
		deployment.Annotations = maputils.Append(deployment.Annotations, constants.WorkspaceRestrictedAccessAnnotation, restrictedAccess)
		deployment.Spec.Template.Annotations = maputils.Append(deployment.Spec.Template.Annotations, constants.WorkspaceRestrictedAccessAnnotation, restrictedAccess)
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
