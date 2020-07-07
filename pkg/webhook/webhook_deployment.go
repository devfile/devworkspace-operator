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
	"github.com/devfile/devworkspace-operator/pkg/config"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"os"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	devworkspaceWebhookServerName = "devworkspace-operator-webhook-server"
)

func CreateWebhookServerDeployment(
	client crclient.Client,
	ctx context.Context,
	namespace string) error {

	deployment := getSpecDeployment(namespace)
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

func getSpecDeployment(namespace string) *appsv1.Deployment {
	replicas := int32(1)
	terminationGracePeriod := int64(1)

	var user *int64
	if !config.ControllerCfg.IsOpenShift() {
		uID := int64(1234)
		user = &uID
	}

	saName := os.Getenv(config.ControllerServiceAccountNameEnvVar)
	if saName == "" {
		return nil
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      devworkspaceWebhookServerName,
			Namespace: namespace,
			Labels:    map[string]string{},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name":    devworkspaceWebhookServerName,
					"app.kubernetes.io/part-of": "devworkspace-operator",
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      devworkspaceWebhookServerName,
					Namespace: namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name":    devworkspaceWebhookServerName,
						"app.kubernetes.io/part-of": "devworkspace-operator",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "webhook-server",
							Image: "quay.io/jpinkney/che-workspace-controller:latest",
							Command: []string{"/usr/local/bin/entrypoint", "/usr/local/bin/webhook-server"},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name: WebhookTLSCertsName,
									MountPath: "/tmp/k8s-webhook-server/serving-certs",
									ReadOnly: true,
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
					AutomountServiceAccountToken: nil,
					Volumes: []corev1.Volume{
						{
							Name: WebhookTLSCertsName,
							VolumeSource: corev1.VolumeSource{
								Projected: &corev1.ProjectedVolumeSource{
									Sources: []corev1.VolumeProjection{
										{
											ConfigMap: &corev1.ConfigMapProjection{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: CertConfigMapName,
												},
												Items: []corev1.KeyToPath{
													{
														Key: "service-ca.crt",
														Path: "./ca.crt",
													},
												},
											},
										},
										{
											Secret: &corev1.SecretProjection{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: SecureServiceName,
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

	return deployment
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
