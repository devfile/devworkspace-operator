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

package che_rest

import (
	k8sModelUtils "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/modelutils/k8s"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"fmt"
	. "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/config"
	. "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/model"
)

func AddCheRestApis(wkspProps WorkspaceProperties, podSpec *corev1.PodSpec) ([]runtime.Object, string, error) {
	cheRestApisPort := 9999
	containerName := "che-rest-apis"
	podSpec.Containers = append(podSpec.Containers, corev1.Container{
		Image:           ControllerCfg.GetCheRestApisDockerImage(),
		ImagePullPolicy: corev1.PullAlways,
		Name:            containerName,
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: int32(cheRestApisPort),
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env: []corev1.EnvVar{
			{
				Name:  "CHE_WORKSPACE_NAME",
				Value: wkspProps.WorkspaceName,
			},
			{
				Name:  "CHE_WORKSPACE_ID",
				Value: wkspProps.WorkspaceId,
			},
			{
				Name:  "CHE_WORKSPACE_NAMESPACE",
				Value: wkspProps.Namespace,
			},
		},
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
	})

	serviceName, servicePort := containerName, k8sModelUtils.ServicePortName(cheRestApisPort)
	serviceNameAndPort := serviceName + "-" + servicePort
	ingressHost := k8sModelUtils.IngressHostName(serviceNameAndPort, wkspProps)
	ingressUrl := "http://" + ingressHost + "/api"

	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        serviceName,
			Namespace:   wkspProps.Namespace,
			Annotations: map[string]string{},
			Labels: map[string]string{
				WorkspaceIDLabel: wkspProps.WorkspaceId,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				CheOriginalNameLabel: CheOriginalName,
				WorkspaceIDLabel:     wkspProps.WorkspaceId,
			},
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       k8sModelUtils.ServicePortName(cheRestApisPort),
					Protocol:   ServicePortProtocol,
					Port:       int32(cheRestApisPort),
					TargetPort: intstr.FromInt(cheRestApisPort),
				},
			},
		},
	}
	ingress := extensionsv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("ingress-%s-%s", wkspProps.WorkspaceId, containerName),
			Namespace: wkspProps.Namespace,
			Annotations: map[string]string{
				"kubernetes.io/ingress.class":                "nginx",
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
				"nginx.ingress.kubernetes.io/ssl-redirect":   "false",
				"org.eclipse.che.machine.name":               containerName,
			},
			Labels: map[string]string{
				CheOriginalNameLabel: serviceNameAndPort,
				WorkspaceIDLabel:     wkspProps.WorkspaceId,
			},
		},
		Spec: extensionsv1beta1.IngressSpec{
			Rules: []extensionsv1beta1.IngressRule{
				{
					IngressRuleValue: extensionsv1beta1.IngressRuleValue{
						HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
							Paths: []extensionsv1beta1.HTTPIngressPath{
								{
									Backend: extensionsv1beta1.IngressBackend{
										ServiceName: serviceName,
										ServicePort: intstr.FromString(servicePort),
									},
								},
							},
						},
					},
				},
			},
		},
	}
	ingress.Spec.Rules[0].Host = ingressHost

	return []runtime.Object{&service, &ingress}, ingressUrl, nil
}
