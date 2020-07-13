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

package webhook

import (
	"context"

	"github.com/devfile/devworkspace-operator/webhook/server"

	"github.com/devfile/devworkspace-operator/internal/images"

	"github.com/devfile/devworkspace-operator/pkg/config"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateWebhookServerDeployment(
	client crclient.Client,
	ctx context.Context,
	namespace string) error {

	deployment, err := getSpecDeployment(namespace)
	if err != nil {
		return err
	}

	if err := client.Create(ctx, deployment); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		existingCfg, err := getClusterDeployment(ctx, namespace, client)
		if err != nil {
			return err
		}
		deployment.ResourceVersion = existingCfg.ResourceVersion
		err = client.Update(ctx, deployment)
		if err != nil {
			return err
		}
		log.Info("Updated webhook server deployment")
	} else {
		log.Info("Created webhook server deployment")
	}
	return nil
}

func getSpecDeployment(namespace string) (*appsv1.Deployment, error) {
	replicas := int32(1)
	terminationGracePeriod := int64(1)
	trueBool := true
	var user *int64
	if !config.ControllerCfg.IsOpenShift() {
		uID := int64(1234)
		user = &uID
	}

	controllerSA, err := config.ControllerCfg.GetWorkspaceControllerSA()
	if err != nil {
		return nil, err
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.WebhookServerDeploymentName,
			Namespace: namespace,
			Labels:    server.WebhookServerAppLabels(),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: server.WebhookServerAppLabels(),
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      server.WebhookServerDeploymentName,
					Namespace: namespace,
					Labels:    server.WebhookServerAppLabels(),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "webhook-server",
							Image:           images.GetWebhookServerImage(),
							Command:         []string{"/usr/local/bin/entrypoint", "/usr/local/bin/webhook-server"},
							ImagePullPolicy: corev1.PullAlways,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      server.WebhookServerCertsVolumeName,
									MountPath: server.WebhookServerCertDir,
									ReadOnly:  true,
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          server.WebhookServerPortName,
									ContainerPort: server.WebhookServerPort,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  config.ControllerServiceAccountNameEnvVar,
									Value: controllerSA,
								},
								{
									Name: "POD_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name: "WATCH_NAMESPACE",
								},
							},
						},
					},
					RestartPolicy:                 "Always",
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser: user,
						FSGroup:   user,
					},
					ServiceAccountName:           server.WebhookServerSAName,
					AutomountServiceAccountToken: &trueBool,
					Volumes: []corev1.Volume{
						{
							Name: server.WebhookServerCertsVolumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: server.WebhookServerTLSSecretName,
								},
							},
						},
					},
				},
			},
		},
	}

	return deployment, nil
}

func getClusterDeployment(ctx context.Context, namespace string, client crclient.Client) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{}
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      server.WebhookServerDeploymentName,
	}
	err := client.Get(ctx, namespacedName, deployment)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return deployment, nil
}
