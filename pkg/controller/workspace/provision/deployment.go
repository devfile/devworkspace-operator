//
// Copyright (c) 2019-2020 Red Hat, Inc.
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
	"fmt"
	"github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	"github.com/che-incubator/che-workspace-operator/pkg/config"
	"github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/env"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"
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
	workspace *v1alpha1.Workspace,
	podAdditions []v1alpha1.PodAdditions,
	saName string,
	clusterAPI ClusterAPI) DeploymentProvisioningStatus {

	// [design] we have to pass components and routing pod additions separately becuase we need mountsources from each
	// component.
	specDeployment, err := getSpecDeployment(workspace, podAdditions, saName, clusterAPI.Scheme)
	if err != nil {
		return DeploymentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err},
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
			if errors.IsConflict(err) {
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

func checkDeploymentStatus(deployment *appsv1.Deployment) (ready bool) {
	return deployment.Status.ReadyReplicas > 0
}

func getSpecDeployment(
	workspace *v1alpha1.Workspace,
	podAdditionsList []v1alpha1.PodAdditions,
	saName string,
	scheme *runtime.Scheme) (*appsv1.Deployment, error) {
	replicas := int32(1)
	terminationGracePeriod := int64(1)
	rollingUpdateParam := intstr.FromInt(1)

	var user *int64
	if !config.ControllerCfg.IsOpenShift() {
		uID := int64(1234)
		user = &uID
	}

	podAdditions, err := mergePodAdditions(podAdditionsList)
	if err != nil {
		return nil, err
	}

	commonEnv := env.CommonEnvironmentVariables(workspace.Name, workspace.Status.WorkspaceId, workspace.Namespace)
	for idx := range podAdditions.Containers {
		podAdditions.Containers[idx].Env = append(podAdditions.Containers[idx].Env, commonEnv...)
	}
	for idx := range podAdditions.InitContainers {
		podAdditions.InitContainers[idx].Env = append(podAdditions.InitContainers[idx].Env, commonEnv...)
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workspace.Status.WorkspaceId,
			Namespace: workspace.Namespace,
			Labels: map[string]string{
				config.WorkspaceIDLabel: workspace.Status.WorkspaceId,
			},
			Annotations: map[string]string{
				config.WorkspaceCreatorAnnotation: workspace.Annotations[config.WorkspaceCreatorAnnotation],
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					config.CheOriginalNameLabel: config.CheOriginalName,
					config.WorkspaceIDLabel:     workspace.Status.WorkspaceId,
					config.WorkspaceNameLabel:   workspace.Name,
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: "RollingUpdate",
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       &rollingUpdateParam,
					MaxUnavailable: &rollingUpdateParam,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      workspace.Status.WorkspaceId,
					Namespace: workspace.Namespace,
					Labels: map[string]string{
						config.CheOriginalNameLabel: config.CheOriginalName,
						config.WorkspaceIDLabel:     workspace.Status.WorkspaceId,
						config.WorkspaceNameLabel:   workspace.Name,
					},
					Annotations: map[string]string{
						config.WorkspaceCreatorAnnotation: workspace.Annotations[config.WorkspaceCreatorAnnotation],
					},
				},
				Spec: corev1.PodSpec{
					InitContainers:                append(podAdditions.InitContainers, precreateSubpathsInitContainer(workspace.Status.WorkspaceId)),
					Containers:                    podAdditions.Containers,
					Volumes:                       append(podAdditions.Volumes, getPersistentVolumeClaim()),
					ImagePullSecrets:              podAdditions.PullSecrets,
					RestartPolicy:                 "Always",
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser: user,
						FSGroup:   user,
					},
					ServiceAccountName:           saName,
					AutomountServiceAccountToken: nil,
				},
			},
		},
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
		if errors.IsNotFound(err) {
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

func getPersistentVolumeClaim() corev1.Volume {
	var workspaceClaim = corev1.PersistentVolumeClaimVolumeSource{
		ClaimName: config.ControllerCfg.GetWorkspacePVCName(),
	}
	pvcVolume := corev1.Volume{
		Name: config.ControllerCfg.GetWorkspacePVCName(),
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &workspaceClaim,
		},
	}
	return pvcVolume
}

func precreateSubpathsInitContainer(workspaceId string) corev1.Container {
	initContainer := corev1.Container{
		Name:    "precreate-subpaths",
		Image:   "registry.access.redhat.com/ubi8/ubi-minimal",
		Command: []string{"/usr/bin/mkdir"},
		Args: []string{
			"-p",
			"-v",
			"-m",
			"777",
			"/tmp/che-workspaces/" + workspaceId,
		},
		ImagePullPolicy: corev1.PullPolicy(config.ControllerCfg.GetSidecarPullPolicy()),
		VolumeMounts: []corev1.VolumeMount{
			{
				MountPath: "/tmp/che-workspaces",
				Name:      config.ControllerCfg.GetWorkspacePVCName(),
				ReadOnly:  false,
			},
		},
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
	}
	return initContainer
}
