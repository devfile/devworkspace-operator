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

package specutils

import (
	"fmt"
	"github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	"github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/config"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
)

const proxyServiceAcctAnnotationKeyFmt string = "serviceaccounts.openshift.io/oauth-redirectreference.%s-%s"
const proxyServiceAcctAnnotationValueFmt string = `{"kind":"OAuthRedirectReference","apiVersion":"v1","reference":{"kind":"Route","name":"%s"}}`

type proxyEndpoint struct {
	name           string
	targetService  string
	targetPort     int64
	proxyHttpsPort int64
}

func ProxyServiceName(basename string) string {
	// TODO: Use workspace ID here?
	return basename + "-oauth-proxy"
}

func proxyServiceAccountName(routingName string) string {
	return routingName + "-oauth-proxy"
}

func proxyDeploymentName(routingName string) string {
	return routingName + "-oauth-proxy"
}

func proxyRouteName(serviceDesc v1alpha1.ServiceDescription, endpoint v1alpha1.Endpoint) string {
	return fmt.Sprintf("%s-%s", serviceDesc.ServiceName, endpoint.Name)
}

func GetProxyServiceAccount(workspaceroutingName, namespace string, service corev1.Service) corev1.ServiceAccount {
	sa := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        proxyServiceAccountName(workspaceroutingName),
			Namespace:   namespace,
			Annotations: map[string]string{},
		},
	}

	for _, servicePort := range service.Spec.Ports {
		port := int64(servicePort.Port)
		key := fmt.Sprintf(proxyServiceAcctAnnotationKeyFmt, workspaceroutingName, strconv.FormatInt(port, 10))
		// TODO : Name proxy service better and fix this
		val := fmt.Sprintf(proxyServiceAcctAnnotationValueFmt, IngressName(service.Name, port))
		sa.Annotations[key] = val
	}
	return sa
}

func GetProxyDeployment(workspaceRoutingName, namespace string, services map[string]v1alpha1.ServiceDescription) appsv1.Deployment {
	var replicas int32 = 1

	var containers []corev1.Container
	for _, endpoint := range getProxyEndpoints(services) {
		containers = append(containers, getProxyDeploymentContainer(workspaceRoutingName, endpoint))
	}

	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      proxyDeploymentName(workspaceRoutingName),
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": proxyDeploymentName(workspaceRoutingName),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": proxyDeploymentName(workspaceRoutingName),
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyAlways,
					ServiceAccountName: proxyServiceAccountName(workspaceRoutingName),
					Volumes:            getProxyVolumes(containers),
					Containers:         containers,
				},
			},
		},
	}
	return deployment
}

func getProxyEndpoints(services map[string]v1alpha1.ServiceDescription) []proxyEndpoint {
	var proxyEndpoints []proxyEndpoint
	for _, serviceDesc := range services {
		for _, endpoint := range serviceDesc.Endpoints {
			if !endpointNeedsProxy(endpoint) {
				continue
			}
			proxyEndpoint := proxyEndpoint{
				name:           proxyRouteName(serviceDesc, endpoint),
				targetService:  serviceDesc.ServiceName,
				targetPort:     endpoint.Port,
				proxyHttpsPort: endpoint.Port,
			}
			proxyEndpoints = append(proxyEndpoints, proxyEndpoint)
		}
	}
	return proxyEndpoints
}

func getProxyDeploymentContainer(saName string, endpoint proxyEndpoint) corev1.Container {
	container := corev1.Container{
		Name: "oauth-proxy-" + endpoint.name,
		Ports: []corev1.ContainerPort{
			{
				Name:          "public",
				ContainerPort: int32(endpoint.proxyHttpsPort),
				Protocol:      corev1.ProtocolTCP,
			},
		},
		ImagePullPolicy: corev1.PullPolicy(config.ControllerCfg.GetSidecarPullPolicy()),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "proxy-tls", // TODO: Can this name be shared among containers or does it have to be unique?
				MountPath: "/etc/tls/private",
			},
		},
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
		Image:                    "openshift/oauth-proxy:latest",
		Args: []string{
			"--https-address=:" + strconv.FormatInt(endpoint.proxyHttpsPort, 10),
			"--http-address=127.0.0.1:" + strconv.Itoa(8080),
			"--provider=openshift",
			"--openshift-service-account=" + proxyServiceAccountName(saName),
			"--upstream=http://" + endpoint.targetService + ":" + strconv.FormatInt(endpoint.targetPort, 10),
			"--tls-cert=/etc/tls/private/tls.crt",
			"--tls-key=/etc/tls/private/tls.key",
			"--cookie-secret=SECRET",
		},
	}

	return container
}

func getProxyVolumes(containers []corev1.Container) []corev1.Volume {
	var volumes []corev1.Volume
	var volumeDefaultMode int32 = 420
	for _, container := range containers {
		for _, volumeMount := range container.VolumeMounts {
			volume := corev1.Volume{
				Name: volumeMount.Name,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  volumeMount.Name,
						DefaultMode: &volumeDefaultMode,
					},
				},
			}
			volumes = append(volumes, volume)
		}
	}
	return volumes
}

func endpointNeedsProxy(endpoint v1alpha1.Endpoint) bool {
	return endpoint.Attributes[v1alpha1.PUBLIC_ENDPOINT_ATTRIBUTE] == "true" &&
		endpoint.Attributes[v1alpha1.SECURE_ENDPOINT_ATTRIBUTE] == "true" &&
		endpoint.Attributes[v1alpha1.TYPE_ENDPOINT_ATTRIBUTE] != "terminal"
}
