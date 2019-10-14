package workspace

import (
	"github.com/eclipse/che-plugin-broker/model"
	"regexp"
	"strconv"

	workspaceApi "github.com/che-incubator/che-workspace-crd-operator/pkg/apis/workspace/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	k8sModelUtils "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/modelutils/k8s"
)

func setupDockerImageComponent(names workspaceProperties, commands []workspaceApi.CommandSpec, component *workspaceApi.ComponentSpec) (*ComponentInstanceStatus, error) {
	componentInstanceStatus := &ComponentInstanceStatus{
		Machines: map[string]MachineDescription{},
		Endpoints: []workspaceApi.Endpoint {},
		ContributedRuntimeCommands: []CheWorkspaceCommand {},
	}

	podTemplate := &corev1.PodTemplateSpec{}
	componentInstanceStatus.WorkspacePodAdditions = podTemplate
	componentInstanceStatus.ExternalObjects = []runtime.Object{}

	var machineName string
	if component.Alias == nil {
		re := regexp.MustCompile(`[^-a-zA-Z0-9_]`)
		machineName = re.ReplaceAllString(*component.Image, "-")
	} else {
		machineName = *component.Alias
	}

	var exposedPorts []int = EndpointPortsToInts(component.Endpoints)

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

	volumeMounts := createVolumeMounts(names, component.MountSources, component.Volumes, []model.Volume{})

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
		Ports:           k8sModelUtils.BuildContainerPorts(exposedPorts, corev1.ProtocolTCP),
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

// TODO selector, etc ....

	podTemplate.Spec.Containers = append(podTemplate.Spec.Containers, container)

	for _, service := range createK8sServicesForMachines(names, machineName, exposedPorts) {
		componentInstanceStatus.ExternalObjects = append(componentInstanceStatus.ExternalObjects, &service)
	}

	machineAttributes := map[string]string {}
	if limitAsInt64, canBeConverted := limit.AsInt64(); canBeConverted {
		machineAttributes[MEMORY_LIMIT_ATTRIBUTE] = strconv.FormatInt(limitAsInt64, 10)
		machineAttributes[MEMORY_REQUEST_ATTRIBUTE] = strconv.FormatInt(limitAsInt64, 10)
	}
	machineAttributes[CONTAINER_SOURCE_ATTRIBUTE] = RECIPE_CONTAINER_SOURCE
	componentInstanceStatus.Machines[machineName] = MachineDescription {
		machineAttributes: machineAttributes,
    ports: exposedPorts,
	}
	
	for _, command := range commands {
		if len(command.Actions) == 0 {
			continue
		}
		action := command.Actions[0]
		if component.Alias == nil ||
		  action.Component != *component.Alias {
			continue
		}
		attributes := map[string]string{
			COMMAND_WORKING_DIRECTORY_ATTRIBUTE: emptyIfNil(action.Workdir),
			COMMAND_ACTION_REFERENCE_ATTRIBUTE:  emptyIfNil(action.Reference),
			COMMAND_ACTION_REFERENCE_CONTENT_ATTRIBUTE:  emptyIfNil(action.ReferenceContent),
			COMMAND_MACHINE_NAME_ATTRIBUTE:      machineName,
			COMPONENT_ALIAS_COMMAND_ATTRIBUTE:   action.Component,
		}
		for attrName, attrValue := range command.Attributes {
			attributes[attrName] = attrValue
		}
		componentInstanceStatus.ContributedRuntimeCommands = append(componentInstanceStatus.ContributedRuntimeCommands,
			CheWorkspaceCommand{
				Name:        command.Name,
				CommandLine: emptyIfNil(action.Command),
				Type:        action.Type,
				Attributes: attributes,
			})
	}

/*
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

// Gérer les 

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
				"org.eclipse.che.machine.source": "recipe",
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

	/*
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
*/

	return componentInstanceStatus, nil
}
