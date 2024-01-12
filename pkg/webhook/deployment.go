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

package webhook

import (
	"context"
	"fmt"

	"github.com/devfile/devworkspace-operator/internal/images"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/devfile/devworkspace-operator/webhook/server"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateWebhookServerDeployment(
	client crclient.Client,
	ctx context.Context,
	webhooksSecretName string,
	namespace string) error {

	deployment, err := getSpecDeployment(webhooksSecretName, namespace)
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

func getSpecDeployment(webhooksSecretName, namespace string) (*appsv1.Deployment, error) {
	var user *int64
	if !infrastructure.IsOpenShift() {
		user = pointer.Int64(1234)
	}

	resources, err := getWebhooksServerResources()
	if err != nil {
		return nil, fmt.Errorf("failed to create webhooks server deployment: %s", err)
	}

	controllerSA, err := config.GetWorkspaceControllerSA()
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
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: server.WebhookServerAppLabels(),
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:        server.WebhookServerDeploymentName,
					Namespace:   namespace,
					Labels:      server.WebhookServerAppLabels(),
					Annotations: server.WebhookServerAppAnnotations(),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "kube-rbac-proxy",
							Image: images.GetKubeRBACProxyImage(),
							Args: []string{
								"--secure-listen-address=0.0.0.0:9443",
								"--upstream=http://127.0.0.1:8080/",
								"--logtostderr=true",
								"--v=10",
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          server.WebhookMetricsPortName,
									ContainerPort: 9443,
								},
							},
						},
						{
							Name:            "webhook-server",
							Image:           images.GetWebhookServerImage(),
							Command:         []string{"/usr/local/bin/entrypoint"},
							Args:            []string{"/usr/local/bin/webhook-server", "--metrics-addr=127.0.0.1:8080"},
							ImagePullPolicy: corev1.PullAlways,
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/healthz",
										Port:   intstr.FromString("liveness-port"),
										Scheme: "HTTP",
									},
								},
								InitialDelaySeconds: 15,
								TimeoutSeconds:      5,
								PeriodSeconds:       10,
								SuccessThreshold:    1,
								FailureThreshold:    5,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/readyz",
										Port:   intstr.FromString("liveness-port"),
										Scheme: "HTTP",
									},
								},
								InitialDelaySeconds: 10,
								TimeoutSeconds:      5,
								PeriodSeconds:       10,
								SuccessThreshold:    1,
								FailureThreshold:    3,
							},
							Resources: *resources,
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
								{
									Name:          "liveness-port",
									ContainerPort: 6789,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  constants.ControllerServiceAccountNameEnvVar,
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
					TerminationGracePeriodSeconds: pointer.Int64(1),
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser: user,
						FSGroup:   user,
					},
					ServiceAccountName:           server.WebhookServerSAName,
					AutomountServiceAccountToken: pointer.Bool(true),
					Volumes: []corev1.Volume{
						{
							Name: server.WebhookServerCertsVolumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: webhooksSecretName,
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

func getWebhooksServerResources() (*corev1.ResourceRequirements, error) {
	memLimit, err := config.GetResourceQuantityFromEnvVar(config.WebhooksMemLimitEnvVar)
	if err != nil {
		return nil, err
	}
	memRequest, err := config.GetResourceQuantityFromEnvVar(config.WebhooksMemRequestEnvVar)
	if err != nil {
		return nil, err
	}
	cpuLimit, err := config.GetResourceQuantityFromEnvVar(config.WebhooksCPULimitEnvVar)
	if err != nil {
		return nil, err
	}
	cpuRequest, err := config.GetResourceQuantityFromEnvVar(config.WebhooksCPURequestEnvVar)
	if err != nil {
		return nil, err
	}
	return &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: *memLimit,
			corev1.ResourceCPU:    *cpuLimit,
		},
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: *memRequest,
			corev1.ResourceCPU:    *cpuRequest,
		},
	}, nil
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
