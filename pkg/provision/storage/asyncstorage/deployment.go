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

package asyncstorage

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/devfile/devworkspace-operator/internal/images"
	wsprovision "github.com/devfile/devworkspace-operator/pkg/provision/workspace"
)

func SyncWorkspaceSyncDeploymentToCluster(namespace string, sshConfigMap *corev1.ConfigMap, storage *corev1.PersistentVolumeClaim, clusterAPI wsprovision.ClusterAPI) (*appsv1.Deployment, error) {
	specDeployment := getWorkspaceSyncDeploymentSpec(namespace, sshConfigMap, storage)
	clusterDeployment, err := GetWorkspaceSyncDeploymentCluster(namespace, clusterAPI)
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			return nil, err
		}
		err := clusterAPI.Client.Create(clusterAPI.Ctx, specDeployment)
		if err != nil && !k8sErrors.IsAlreadyExists(err) {
			return nil, err
		}
		return nil, NotReadyError
	}
	if !equality.Semantic.DeepDerivative(specDeployment.Spec, clusterDeployment.Spec) {
		err := clusterAPI.Client.Patch(clusterAPI.Ctx, specDeployment, client.Merge)
		if err != nil && !k8sErrors.IsConflict(err) {
			return nil, err
		}
		return nil, NotReadyError
	}
	if clusterDeployment.Status.ReadyReplicas > 0 {
		return clusterDeployment, nil
	}
	return nil, NotReadyError
}

func getWorkspaceSyncDeploymentSpec(namespace string, sshConfigMap *corev1.ConfigMap, storage *corev1.PersistentVolumeClaim) *appsv1.Deployment {
	replicas := int32(1)
	terminationGracePeriod := int64(1)
	modeReadOnly := int32(0640)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      asyncServerDeploymentName,
			Namespace: namespace,
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
					Namespace: namespace,
					Labels:    asyncServerLabels,
				},
				Spec: corev1.PodSpec{
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
									ClaimName: storage.Name,
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
					SecurityContext:               wsprovision.GetDevWorkspaceSecurityContext(),
					AutomountServiceAccountToken:  nil,
				},
			},
		},
	}
	return deployment
}

func GetWorkspaceSyncDeploymentCluster(namespace string, clusterAPI wsprovision.ClusterAPI) (*appsv1.Deployment, error) {
	deploy := &appsv1.Deployment{}
	namespacedName := types.NamespacedName{
		Name:      "async-storage", // TODO
		Namespace: namespace,
	}
	err := clusterAPI.Client.Get(clusterAPI.Ctx, namespacedName, deploy)
	return deploy, err
}
