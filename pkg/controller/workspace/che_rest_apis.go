package workspace

import (
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func addCheRestApis(wkspProps workspaceProperties, podSpec *corev1.PodSpec) ([]runtime.Object, string, error) {
	cheRestApisPort := 9999
	containerName := "che-rest-apis"
	podSpec.Containers = append(podSpec.Containers, corev1.Container{
		Image:           "dfestal/che-rest-apis",
		ImagePullPolicy: corev1.PullIfNotPresent,
		Name:            containerName,
		Ports: []corev1.ContainerPort{
			corev1.ContainerPort{
				ContainerPort: int32(cheRestApisPort),
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env: []corev1.EnvVar{
			corev1.EnvVar{
				Name:  "CHE_WORKSPACE_NAME",
				Value: wkspProps.workspaceName,
			},
			corev1.EnvVar{
				Name:  "CHE_WORKSPACE_ID",
				Value: wkspProps.workspaceId,
			},
			corev1.EnvVar{
				Name:  "CHE_WORKSPACE_NAMESPACE",
				Value: wkspProps.namespace,
			},
		},
	})

	serviceName, servicePort := containerName, servicePortName(cheRestApisPort)
	serviceNameAndPort := join("-", serviceName, servicePort)
	ingressHost := join(".", serviceNameAndPort, workspaceConfig.getIngressGlobalDomain())
	ingressUrl := "http://" + ingressHost + "/api"

	return []runtime.Object{
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        serviceName,
				Namespace:   wkspProps.namespace,
				Annotations: map[string]string{},
				Labels: map[string]string{
					"che.workspace_id": wkspProps.workspaceId,
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"che.original_name": cheOriginalName,
					"che.workspace_id":  wkspProps.workspaceId,
				},
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					corev1.ServicePort{
						Name:       servicePortName(cheRestApisPort),
						Protocol:   servicePortProtocol,
						Port:       int32(cheRestApisPort),
						TargetPort: intstr.FromInt(cheRestApisPort),
					},
				},
			},
		},
		&extensionsv1beta1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      join("-", "ingress", wkspProps.workspaceId, containerName),
				Namespace: wkspProps.namespace,
				Annotations: map[string]string{
					"kubernetes.io/ingress.class":                "nginx",
					"nginx.ingress.kubernetes.io/rewrite-target": "/",
					"nginx.ingress.kubernetes.io/ssl-redirect":   "false",
					"org.eclipse.che.machine.name":               containerName,
				},
				Labels: map[string]string{
					"che.original_name": serviceNameAndPort,
					"che.workspace_id":  wkspProps.workspaceId,
				},
			},
			Spec: extensionsv1beta1.IngressSpec{
				Rules: []extensionsv1beta1.IngressRule{
					extensionsv1beta1.IngressRule{
						Host: ingressHost,
						IngressRuleValue: extensionsv1beta1.IngressRuleValue{
							HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
								Paths: []extensionsv1beta1.HTTPIngressPath{
									extensionsv1beta1.HTTPIngressPath{
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
		},
	}, ingressUrl, nil
}
