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

package asyncstorage

import (
	"github.com/devfile/devworkspace-operator/internal/images"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	nsconfig "github.com/devfile/devworkspace-operator/pkg/provision/config"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func SyncWorkspaceSyncDeploymentToCluster(workspace *common.DevWorkspaceWithConfig, sshConfigMap *corev1.ConfigMap, pvcName string, clusterAPI sync.ClusterAPI) (*appsv1.Deployment, error) {
	podTolerations, nodeSelector, err := nsconfig.GetNamespacePodTolerationsAndNodeSelector(workspace.Namespace, clusterAPI)
	if err != nil {
		return nil, err
	}

	specDeployment := getWorkspaceSyncDeploymentSpec(workspace, sshConfigMap, pvcName, podTolerations, nodeSelector)
	clusterObj, err := sync.SyncObjectWithCluster(specDeployment, clusterAPI)
	switch err.(type) {
	case nil:
		break
	case *sync.NotInSyncError:
		return nil, NotReadyError
	case *sync.UnrecoverableSyncError:
		return nil, err // TODO: This should fail workspace start
	default:
		return nil, err
	}

	clusterDeployment := clusterObj.(*appsv1.Deployment)
	if clusterDeployment.Status.ReadyReplicas > 0 {
		return clusterDeployment, nil
	}
	return nil, NotReadyError
}

func getWorkspaceSyncDeploymentSpec(
	workspace *common.DevWorkspaceWithConfig,
	sshConfigMap *corev1.ConfigMap,
	pvcName string,
	tolerations []corev1.Toleration,
	nodeSelector map[string]string) *appsv1.Deployment {

	replicas := int32(1)
	terminationGracePeriod := int64(1)
	modeReadOnly := int32(0640)

	var securityContext *corev1.PodSecurityContext
	if infrastructure.IsOpenShift() {
		securityContext = &corev1.PodSecurityContext{}
	} else {
		securityContext = workspace.Config.Workspace.PodSecurityContext
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      asyncServerDeploymentName,
			Namespace: workspace.Namespace,
			Labels:    asyncServerLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: asyncServerLabels,
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "async-storage-server",
					Namespace: workspace.Namespace,
					Labels:    asyncServerLabels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyAlways,
					Containers: []corev1.Container{
						{
							Name:  "async-storage-server",
							Image: images.GetAsyncStorageServerImage(),
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: rsyncPort,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceMemory: resource.MustParse(asyncServerMemoryLimit),
								},
								Requests: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceMemory: resource.MustParse(asyncServerMemoryRequest),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "async-storage-data",
									MountPath: "/async-storage",
								},
								{
									// TODO: mounting a configmap with SubPath prevents changes from being propagated into the
									// container and not using a subpath replaces all files in the directory and mounts it as a
									// read-only filesystem.
									// As a workaround, we could mount the whole configmap to some other directory and copy
									// the file on startup, but this would require changes in the che-workspace-data-sync-storage
									// container
									// See issue https://github.com/kubernetes/kubernetes/issues/50345 for more info
									Name:      "async-storage-config",
									MountPath: "/.ssh/authorized_keys",
									ReadOnly:  true,
									SubPath:   "authorized_keys",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "async-storage-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: pvcName,
								},
							},
						},
						{
							Name: "async-storage-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: sshConfigMap.Name,
									},
									DefaultMode: &modeReadOnly,
								},
							},
						},
					},
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					SecurityContext:               securityContext,
					AutomountServiceAccountToken:  nil,
				},
			},
		},
	}

	if tolerations != nil && len(tolerations) > 0 {
		deployment.Spec.Template.Spec.Tolerations = tolerations
	}

	if nodeSelector != nil && len(nodeSelector) > 0 {
		deployment.Spec.Template.Spec.NodeSelector = nodeSelector
	}

	return deployment
}

func GetWorkspaceSyncDeploymentCluster(namespace string, clusterAPI sync.ClusterAPI) (*appsv1.Deployment, error) {
	deploy := &appsv1.Deployment{}
	namespacedName := types.NamespacedName{
		Name:      "async-storage", // TODO
		Namespace: namespace,
	}
	err := clusterAPI.Client.Get(clusterAPI.Ctx, namespacedName, deploy)
	return deploy, err
}
