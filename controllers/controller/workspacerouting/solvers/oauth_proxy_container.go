//
// Copyright (c) 2019-2021 Red Hat, Inc.
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

	"github.com/devfile/devworkspace-operator/internal/images"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/config"
	corev1 "k8s.io/api/core/v1"
)

func getProxyPodAdditions(proxyEndpoints map[string]proxyEndpoint, meta WorkspaceMetadata) *controllerv1alpha1.PodAdditions {
	tlsSecretVolume := buildSecretVolume(common.OAuthProxySecretName(meta.WorkspaceId))
	var proxyContainers []corev1.Container
	for _, proxyEndpoint := range proxyEndpoints {
		proxyContainers = append(proxyContainers, getProxyContainerForEndpoint(proxyEndpoint, tlsSecretVolume, meta))
	}
	return &controllerv1alpha1.PodAdditions{
		Containers: proxyContainers,
		Volumes:    []corev1.Volume{tlsSecretVolume},
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
	upstreamPortString := strconv.FormatInt(int64(proxyEndpoint.upstreamEndpoint.TargetPort), 10)
	containerPortString := strconv.FormatInt(int64(proxyEndpoint.publicEndpoint.TargetPort), 10)
	proxyContainerName := fmt.Sprintf("oauth-proxy-%s-%s", upstreamPortString, containerPortString)

	return corev1.Container{
		Name: proxyContainerName,
		Ports: []corev1.ContainerPort{
			{
				//Name:          endpoint.upstreamEndpoint.Name,
				ContainerPort: int32(proxyEndpoint.publicEndpoint.TargetPort),
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
		Image:                    images.GetOpenShiftOAuthProxyImage(),
		Args: []string{
			"--https-address=:" + containerPortString,
			"--http-address=127.0.0.1:" + strconv.FormatInt(int64(proxyEndpoint.publicEndpointHttpPort), 10),
			"--provider=openshift",
			"--upstream=http://localhost:" + upstreamPortString,
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
