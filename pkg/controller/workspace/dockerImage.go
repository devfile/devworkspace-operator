package workspace

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"

	workspaceApi "github.com/che-incubator/che-workspace-crd-operator/pkg/apis/workspace/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func setupDockerImageComponent(names workspaceProperties, component *workspaceApi.ComponentSpec, podSpec *corev1.PodSpec) ([]runtime.Object, error) {
	var k8sObjects []runtime.Object

	var containerPorts []corev1.ContainerPort
	var servicePorts []corev1.ServicePort
	for _, endpointDef := range component.Endpoints {
		containerPorts = append(containerPorts, corev1.ContainerPort{
			ContainerPort: int32(endpointDef.Port),
			Protocol:      corev1.ProtocolTCP,
		})
		servicePorts = append(servicePorts, corev1.ServicePort{
			Name:       servicePortName(int(endpointDef.Port)),
			Protocol:   servicePortProtocol,
			Port:       int32(endpointDef.Port),
			TargetPort: intstr.FromInt(int(endpointDef.Port)),
		})
	}

	var machineName string
	if component.Alias == nil {
		re := regexp.MustCompile(`[^-a-zA-Z0-9_]`)
		machineName = re.ReplaceAllString(*component.Image, "-")
	} else {
		machineName = *component.Alias
	}
	var limitOrDefault string

	if *component.MemoryLimit == "" {
		limitOrDefault = "128M"
	} else {
		limitOrDefault = *component.MemoryLimit
	}

	limit, err := resource.ParseQuantity(limitOrDefault)
	if err != nil {
		return nil, err
	}

	var volumeMounts []corev1.VolumeMount

	for _, volDef := range component.Volumes {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			MountPath: volDef.ContainerPath,
			Name:      "claim-che-workspace",
			SubPath:   names.workspaceId + "/" + volDef.Name + "/",
		})
	}

	if component.MountSources != nil && *component.MountSources {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			MountPath: "/projects",
			Name:      "claim-che-workspace",
			SubPath:   names.workspaceId + "/projects/",
		})
	}

	var envVars []corev1.EnvVar
	for _, envVarDef := range component.Env {
		envVars = append(envVars, corev1.EnvVar{
			Name:  envVarDef.Name,
			Value: envVarDef.Value,
		})
	}
	envVars = append(envVars, corev1.EnvVar{
		Name:  "CHE_MACHINE_NAME",
		Value: machineName,
	})
	container := corev1.Container{
		Name:            machineName,
		Image:           *component.Image,
		ImagePullPolicy: defaultImagePullPolicy,
		Ports:           containerPorts,
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				"memory": limit,
			},
			Requests: corev1.ResourceList{
				"memory": limit,
			},
		},
		VolumeMounts: volumeMounts,
		Env:          append(envVars, commonEnvironmentVariables(names)...),
	}
	if component.Command != nil {
		container.Command = *component.Command
	}
	if component.Args != nil {
		container.Args = *component.Args
	}

	podSpec.Containers = append(podSpec.Containers, container)

	if len(component.Endpoints) == 0 {
		return k8sObjects, nil
	}

	var serviceName string
	alreadyOneEndpointDiscoverable := false
	for _, endpointDef := range component.Endpoints {
		if endpointDef.Attributes != nil &&
			endpointDef.Attributes.Discoverable != nil &&
			*endpointDef.Attributes.Discoverable {
			if alreadyOneEndpointDiscoverable {
				return []runtime.Object{}, errors.New("There should be only 1 discoverable endpoint for a dockerImage component")
			}
			serviceName = endpointDef.Name
			alreadyOneEndpointDiscoverable = true
		}
	}

	if serviceName == "" {
		serviceName = join("-",
			"server",
			strings.ReplaceAll(names.workspaceId, "workspace", ""),
			cheOriginalName,
			machineName)
	}

	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: names.namespace,
			Annotations: map[string]string{
				"org.eclipse.che.machine.name":   machineName,
				"org.eclipse.che.machine.source": "component",
				// TODO : do we need to add that it comes from a dockerImage component ???
			},
			Labels: map[string]string{
				"che.workspace_id": names.workspaceId,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"che.original_name": cheOriginalName,
				"che.workspace_id":  names.workspaceId,
			},
			Type:  corev1.ServiceTypeClusterIP,
			Ports: servicePorts,
		},
	}
	k8sObjects = append(k8sObjects, &service)

	for _, endpointDef := range component.Endpoints {
		port := int(endpointDef.Port)

		serverAnnotationName := func(attrName string) string {
			return join(".", "org.eclipse.che.server", attrName)
		}
		protocol := "http"
		serverAnnotationAttributes := func() string {
			attrMap := map[string]string{}
			if attributes := endpointDef.Attributes; attributes != nil {
				if attributes.Discoverable != nil {
					attrMap["discoverable"] = strconv.FormatBool(*attributes.Discoverable)
				}
				if attributes.Path != nil {
					attrMap["path"] = *attributes.Path
				}
				if attributes.Public != nil {
					attrMap["public"] = strconv.FormatBool(*attributes.Public)
					attrMap["internal"] = strconv.FormatBool(!*attributes.Public)
				}
				if attributes.Secure != nil {
					attrMap["secure"] = strconv.FormatBool(*attributes.Secure)
				}
				if attributes.Protocol != nil {
					protocol = *attributes.Protocol
				}
				res, err := json.Marshal(attrMap)
				if err != nil {
					return "{}"
				}
				return string(res)
			}
			return "{}"
		}

		serviceName, servicePort := service.Name, servicePortName(port)
		serviceNameAndPort := join("-", serviceName, servicePort)

		ingress := extensionsv1beta1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      join("-", "ingress", names.workspaceId, endpointDef.Name),
				Namespace: names.namespace,
				Annotations: map[string]string{
					"kubernetes.io/ingress.class":                "nginx",
					"nginx.ingress.kubernetes.io/rewrite-target": "/",
					"nginx.ingress.kubernetes.io/ssl-redirect":   "false",
					"org.eclipse.che.machine.name":               machineName,
					serverAnnotationName("attributes"):           serverAnnotationAttributes(),
					serverAnnotationName("port"):                 servicePortAndProtocol(port),
					serverAnnotationName("protocol"):             protocol,
				},
				Labels: map[string]string{
					"che.original_name": serviceNameAndPort,
					"che.workspace_id":  names.workspaceId,
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
		ingress.Spec.Rules[0].Host = ingressHostName(serviceNameAndPort, names)

		k8sObjects = append(k8sObjects, &ingress)
	}

	return k8sObjects, nil
}
