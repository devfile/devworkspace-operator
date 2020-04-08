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

package solvers

import (
	"fmt"
	"strconv"

	"github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	"github.com/che-incubator/che-workspace-operator/pkg/common"
	"github.com/che-incubator/che-workspace-operator/pkg/config"
	corev1 "k8s.io/api/core/v1"
)

const proxyServiceAcctAnnotationKeyFmt string = "serviceaccounts.openshift.io/oauth-redirectreference.%s-%s"
const proxyServiceAcctAnnotationValueFmt string = `{"kind":"OAuthRedirectReference","apiVersion":"v1","reference":{"kind":"Route","name":"%s"}}`

func getProxyPodAdditions(proxyEndpoints map[string]proxyEndpoint, meta WorkspaceMetadata) *v1alpha1.PodAdditions {
	tlsSecretVolume := buildSecretVolume(common.OAuthProxySecretName(meta.WorkspaceId))
	var proxyContainers []corev1.Container
	for _, proxyEndpoint := range proxyEndpoints {
		proxyContainers = append(proxyContainers, getProxyContainerForEndpoint(proxyEndpoint, tlsSecretVolume, meta))
	}
	serviceAcctAnnotations := getProxyServiceAcctAnnotations(proxyEndpoints, meta)

	return &v1alpha1.PodAdditions{
		Containers:                proxyContainers,
		Volumes:                   []corev1.Volume{tlsSecretVolume},
		ServiceAccountAnnotations: serviceAcctAnnotations,
	}
}

func buildSecretVolume(secretName string) corev1.Volume {
	var readOnly int32 = 420
	return corev1.Volume{
		Name: secretName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  secretName,
				DefaultMode: &readOnly,
			},
		},
	}
}

func getProxyContainerForEndpoint(proxyEndpoint proxyEndpoint, tlsProxyVolume corev1.Volume, meta WorkspaceMetadata) corev1.Container {
	proxyContainerName := fmt.Sprintf("oauth-proxy-%s", strconv.FormatInt(proxyEndpoint.upstreamEndpoint.Port, 10))

	return corev1.Container{
		Name: proxyContainerName,
		Ports: []corev1.ContainerPort{
			{
				//Name:          endpoint.upstreamEndpoint.Name,
				ContainerPort: int32(proxyEndpoint.publicEndpoint.Port),
				Protocol:      corev1.ProtocolTCP,
			},
		},
		ImagePullPolicy: corev1.PullPolicy(config.ControllerCfg.GetSidecarPullPolicy()),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      tlsProxyVolume.Name,
				MountPath: "/etc/tls/private",
			},
		},
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
		Image:                    "openshift/oauth-proxy:latest",
		Args: []string{
			"--https-address=:" + strconv.FormatInt(proxyEndpoint.publicEndpoint.Port, 10),
			"--http-address=127.0.0.1:" + strconv.FormatInt(proxyEndpoint.publicEndpointHttpPort, 10),
			"--provider=openshift",
			"--upstream=http://localhost:" + strconv.FormatInt(proxyEndpoint.upstreamEndpoint.Port, 10),
			"--tls-cert=/etc/tls/private/tls.crt",
			"--tls-key=/etc/tls/private/tls.key",
			"--cookie-secret=0123456789abcdefabcd",
			"--client-id=" + meta.WorkspaceId + "-oauth-client",
			"--client-secret=1234567890",
			"--pass-user-bearer-token=false",
			"--pass-access-token=true",
			"--scope=user:full",
		},
	}
}

func getProxyServiceAcctAnnotations(proxyEndpoints map[string]proxyEndpoint, meta WorkspaceMetadata) map[string]string {
	annotations := map[string]string{}

	for _, proxyEndpoint := range proxyEndpoints {
		portNum := proxyEndpoint.publicEndpoint.Port
		routeName := common.RouteName(meta.WorkspaceId, proxyEndpoint.publicEndpoint.Name)
		annotKey := fmt.Sprintf(proxyServiceAcctAnnotationKeyFmt, meta.WorkspaceId, strconv.FormatInt(portNum, 10))
		annotVal := fmt.Sprintf(proxyServiceAcctAnnotationValueFmt, routeName)
		annotations[annotKey] = annotVal
	}

	return annotations
}
