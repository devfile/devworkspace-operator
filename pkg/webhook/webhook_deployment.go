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

const (
	devworkspaceWebhookServerName = "devworkspace-operator-webhook-server"
)

func CreateWebhookServerDeployment(
	client crclient.Client,
	ctx context.Context,
	namespace string,
	saName string) error {

	deployment, err := getSpecDeployment(namespace, saName)
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

func getSpecDeployment(namespace string, saName string) (*appsv1.Deployment, error) {
	replicas := int32(1)
	terminationGracePeriod := int64(1)
	trueBool := true
	var user *int64
	if !config.ControllerCfg.IsOpenShift() {
		uID := int64(1234)
		user = &uID
	}

	labels := map[string]string{
		"app.kubernetes.io/name":    "devworkspace-webhook-server",
		"app.kubernetes.io/part-of": "devworkspace-operator",
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      devworkspaceWebhookServerName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Strategy: appsv1.DeploymentStrategy{
				//TODO Can it be RollingUpdate?
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      devworkspaceWebhookServerName,
					Namespace: namespace,
					Labels:    labels,
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
									Name:      server.WebhookCertsVolumeName,
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
									Value: saName,
								},
								{
									Name: "POD_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
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
					ServiceAccountName:           saName,
					AutomountServiceAccountToken: &trueBool,
					Volumes: []corev1.Volume{
						{
							Name: server.WebhookCertsVolumeName,
							VolumeSource: corev1.VolumeSource{
								Projected: &corev1.ProjectedVolumeSource{
									Sources: []corev1.VolumeProjection{
										{
											ConfigMap: &corev1.ConfigMapProjection{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: server.CertConfigMapName,
												},
												Items: []corev1.KeyToPath{
													{
														Key:  "service-ca.crt",
														Path: "./ca.crt",
													},
												},
											},
										},
										{
											Secret: &corev1.SecretProjection{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: server.CertSecretName,
												},
											},
										},
									},
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
		Name:      devworkspaceWebhookServerName,
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
