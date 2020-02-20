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

const proxyServiceAcctAnnotationKeyFmt string = "serviceaccounts.openshift.io/oauth-redirectreference.%s"
const proxyServiceAcctAnnotationValueFmt string = `{"kind":"OAuthRedirectReference","apiVersion":"v1","reference":{"kind":"Route","name":"%s"}}`

type proxyEndpoint struct {
	index          int
	targetService  string
	targetPort     int64
	proxyHttpPort  int
	proxyHttpsPort int
}

func ProxyServiceName(serviceName string, port int64) string {
	return IngressName(serviceName, port) + "-oauth-proxy"
}

func ProxyServiceAccountName(routingName string) string {
	return routingName + "-oauth-proxy"
}

func ProxyDeploymentName(routingName string) string {
	return routingName + "-oauth-proxy"
}

func GetProxyServiceAccount(workspaceroutingName, namespace string, proxyEndpoints []proxyEndpoint) corev1.ServiceAccount {
	sa := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ProxyServiceAccountName(workspaceroutingName),
			Namespace:   namespace,
			Annotations: map[string]string{},
		},
	}

	for _, endpoint := range proxyEndpoints {
		key := fmt.Sprintf(proxyServiceAcctAnnotationKeyFmt, strconv.Itoa(endpoint.index))
		val := fmt.Sprintf(proxyServiceAcctAnnotationValueFmt, IngressName(endpoint.targetService, endpoint.targetPort))
		sa.Annotations[key] = val
	}

	return sa
}

func GetProxyDeployment(workspaceRoutingName, namespace string, services map[string]v1alpha1.ServiceDescription) (appsv1.Deployment, corev1.ServiceAccount) {
	var replicas int32 = 1

	var containers []corev1.Container
	proxyEndpoints := getProxyEndpoints(services)
	serviceAcct := GetProxyServiceAccount(workspaceRoutingName, namespace, proxyEndpoints)
	for _, endpoint := range getProxyEndpoints(services) {
		containers = append(containers, getProxyDeploymentContainer(workspaceRoutingName, endpoint))
	}

	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ProxyDeploymentName(workspaceRoutingName),
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": ProxyDeploymentName(workspaceRoutingName),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": ProxyDeploymentName(workspaceRoutingName),
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyAlways,
					ServiceAccountName: ProxyServiceAccountName(workspaceRoutingName),
					Volumes:            getProxyVolumes(containers),
					Containers:         containers,
				},
			},
		},
	}
	return deployment, serviceAcct
}

func getProxyEndpoints(services map[string]v1alpha1.ServiceDescription) []proxyEndpoint {
	var proxyEndpoints []proxyEndpoint
	index := 0
	initialProxyHttpPort := 4180
	initialProxyHttpsPort := 8443
	for _, serviceDesc := range services {
		for _, endpoint := range serviceDesc.Endpoints {
			if !endpointNeedsProxy(endpoint) {
				continue
			}

			proxyEndpoint := proxyEndpoint{
				index:          index,
				targetService:  serviceDesc.ServiceName,
				targetPort:     endpoint.Port,
				proxyHttpPort:  initialProxyHttpPort + index,
				proxyHttpsPort: initialProxyHttpsPort + index,
			}
			proxyEndpoints = append(proxyEndpoints, proxyEndpoint)
			index++
		}
	}
	return proxyEndpoints
}

func getProxyDeploymentContainer(saName string, endpoint proxyEndpoint) corev1.Container {
	container := corev1.Container{
		Name: "oauth-proxy-" + strconv.Itoa(endpoint.index),
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
				Name:      "proxy-tls" + strconv.Itoa(endpoint.index),
				MountPath: "/etc/tls/private",
			},
		},
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
		Image:                    "openshift/oauth-proxy:latest",
		Args: []string{
			"--https-address=:" + strconv.Itoa(endpoint.proxyHttpsPort),
			"--http-address=127.0.0.1:" + strconv.Itoa(endpoint.proxyHttpPort),
			"--provider=openshift",
			"--openshift-service-account=" + ProxyServiceAccountName(saName),
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
