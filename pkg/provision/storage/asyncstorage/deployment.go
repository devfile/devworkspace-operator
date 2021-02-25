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
	"github.com/devfile/devworkspace-operator/controllers/workspace/provision"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SyncWorkspaceSyncDeploymentToCluster(namespace string, sshConfigMap *corev1.ConfigMap, storage *corev1.PersistentVolumeClaim, clusterAPI provision.ClusterAPI) (*appsv1.Deployment, error) {
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
	modeReadOnly := int32(416) // 0640 (octal) in base-10

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "async-storage", // TODO
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":    "async-storage", // TODO
				"app.kubernetes.io/part-of": "devworkspace-operator",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"controller.devfile.io/component": "async-storage", // TODO
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "async-storage-server",
					Namespace: namespace,
					Labels: map[string]string{
						"controller.devfile.io/component": "async-storage",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "async-storage-server",
							Image: "quay.io/eclipse/che-workspace-data-sync-storage:0.0.1",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: sshServerPort,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							// TODO: resources
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "async-storage-data",
									MountPath: "/async-storage",
								},
								{
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
					SecurityContext:               provision.GetDevWorkspaceSecurityContext(),
					//ServiceAccountName:            saName, // TODO
					AutomountServiceAccountToken: nil,
				},
			},
		},
	}
	return deployment
}

func GetWorkspaceSyncDeploymentCluster(namespace string, clusterAPI provision.ClusterAPI) (*appsv1.Deployment, error) {
	deploy := &appsv1.Deployment{}
	namespacedName := types.NamespacedName{
		Name:      "async-storage", // TODO
		Namespace: namespace,
	}
	err := clusterAPI.Client.Get(clusterAPI.Ctx, namespacedName, deploy)
	return deploy, err
}
