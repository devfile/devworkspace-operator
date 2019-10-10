package workspace

import (
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	k8sModelUtils "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/modelutils/k8s"
)

func addCheRestApis(wkspProps workspaceProperties, podSpec *corev1.PodSpec) ([]runtime.Object, string, error) {
	cheRestApisPort := 9999
	containerName := "che-rest-apis"
	podSpec.Containers = append(podSpec.Containers, corev1.Container{
		Image:           controllerConfig.getCheRestApisDockerImage(),
		ImagePullPolicy: corev1.PullAlways,
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
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
	})

	serviceName, servicePort := containerName, k8sModelUtils.ServicePortName(cheRestApisPort)
	serviceNameAndPort := join("-", serviceName, servicePort)
	ingressHost := ingressHostName(serviceNameAndPort, wkspProps)
	ingressUrl := "http://" + ingressHost + "/api"

	service := corev1.Service{
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
					Name:       k8sModelUtils.ServicePortName(cheRestApisPort),
					Protocol:   servicePortProtocol,
					Port:       int32(cheRestApisPort),
					TargetPort: intstr.FromInt(cheRestApisPort),
				},
			},
		},
	}
	ingress := extensionsv1beta1.Ingress{
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
	}
	ingress.Spec.Rules[0].Host = ingressHost

	return []runtime.Object{&service, &ingress}, ingressUrl, nil
}
