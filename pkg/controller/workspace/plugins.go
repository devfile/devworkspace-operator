package workspace

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/eclipse/che-plugin-broker/utils"

	workspaceApi "github.com/che-incubator/che-workspace-crd-operator/pkg/apis/workspace/v1alpha1"
	k8sModelUtils "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/modelutils/k8s"
	pluginModelUtils "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/modelutils/plugins"
	metadataBroker "github.com/eclipse/che-plugin-broker/brokers/metadata"
	commonBroker "github.com/eclipse/che-plugin-broker/common"
	"github.com/eclipse/che-plugin-broker/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TODO : change this because we don't expect plugin metas anymore, but plugin FQNs in the config maps
func setupPluginInitContainers(names workspaceProperties, podSpec *corev1.PodSpec, pluginFQNs []model.PluginFQN) ([]runtime.Object, error) {
	var k8sObjects []runtime.Object

	type initContainerDef struct {
		imageName  string
		pluginFQNs []model.PluginFQN
	}

	for _, def := range []initContainerDef{
		initContainerDef{
			imageName:  "che.workspace.plugin_broker.init.image",
			pluginFQNs: []model.PluginFQN{},
		},
		initContainerDef{
			imageName:  "che.workspace.plugin_broker.unified.image",
			pluginFQNs: pluginFQNs,
		},
	} {
		brokerImage := controllerConfig.getProperty(def.imageName)
		if brokerImage == nil {
			return nil, errors.New("Unknown broker docker image for : " + def.imageName)
		}

		volumeMounts := []corev1.VolumeMount{
			corev1.VolumeMount{
				MountPath: "/plugins/",
				Name:      "claim-che-workspace",
				SubPath:   names.workspaceId + "/plugins/",
			},
		}

		containerName := strings.ReplaceAll(
			strings.TrimSuffix(
				strings.TrimPrefix(def.imageName, "che.workspace.plugin_"),
				".image"),
			".", "-")
		args := []string{
			"-disable-push",
			"-runtime-id",
			join(":",
				names.workspaceId,
				"default",
				"anonymous",
			),
			"--registry-address",
			controllerConfig.getPluginRegistry(),
		}

		if len(def.pluginFQNs) > 0 {

			// TODO: Voir comment le unified broker est défini dans le yaml
			// et le définir de la même manière ici.
			// Voir aussi comment ça se fait qu'on ne met pas de volume =>
			//    => log du côté operator pour voir ce qu'il y a dans les PluginFQNs

			configMapName := containerName + "-broker-config-map"
			configMapVolume := containerName + "-broker-config-volume"
			configMapContent, err := json.MarshalIndent(pluginFQNs, "", "")
			if err != nil {
				return nil, err
			}

			configMap := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: names.namespace,
					Labels: map[string]string{
						"che.workspace_id": names.workspaceId,
					},
				},
				Data: map[string]string{
					"config.json": string(configMapContent),
				},
			}
			k8sObjects = append(k8sObjects, &configMap)

			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				MountPath: "/broker-config/",
				Name:      configMapVolume,
				ReadOnly:  true,
			})

			podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
				Name: configMapVolume,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: configMapName,
						},
					},
				},
			})

			args = append(args,
				"-metas",
				"/broker-config/config.json",
			)
		}

		podSpec.InitContainers = append(podSpec.InitContainers, corev1.Container{
			Name:  containerName,
			Image: *brokerImage,
			Args:  args,

			ImagePullPolicy:          corev1.PullIfNotPresent,
			VolumeMounts:             volumeMounts,
			TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
		})
	}
	return k8sObjects, nil
}

func setupChePlugin(names workspaceProperties, component *workspaceApi.ComponentSpec) (*ComponentInstanceStatus, error) {
	theIoUtil := utils.New()
	theRand := commonBroker.NewRand()

	pluginFQN := model.PluginFQN{}
	idParts := strings.Split(*component.Id, "/")
	idPartsLen := len(idParts)
	if idPartsLen < 3 {
		return nil, errors.New("Invalid component ID: " + *component.Id)
	}
	pluginFQN.ID = strings.Join(idParts[idPartsLen-3:3], "/")
	if idPartsLen > 3 {
		pluginFQN.Registry = strings.Join(idParts[0:idPartsLen-3], "/")
	}

	pluginMeta, err := utils.GetPluginMeta(pluginFQN, controllerConfig.getPluginRegistry(), theIoUtil)
	if err != nil {
		return nil, err
	}

	pluginMetas := []model.PluginMeta{*pluginMeta}
	err = utils.ResolveRelativeExtensionPaths(pluginMetas, controllerConfig.getPluginRegistry())
	if err != nil {
		return nil, err
	}
	err = utils.ValidateMetas(pluginMetas...)
	if err != nil {
		return nil, err
	}
	pluginMeta = &pluginMetas[0]

	chePlugin := metadataBroker.ConvertMetaToPlugin(*pluginMeta)

	isTheiaOrVsCodePlugin := utils.IsTheiaOrVscodePlugin(*pluginMeta)

	if isTheiaOrVsCodePlugin && len(pluginMeta.Spec.Containers) > 0 {
		metadataBroker.AddPluginRunnerRequirements(chePlugin, theRand, true)
	}

	componentInstanceStatus := &ComponentInstanceStatus{
		Machines:                   map[string]MachineDescription{},
		Endpoints:                  []workspaceApi.Endpoint{},
		ContributedRuntimeCommands: []CheWorkspaceCommand{},
		PluginFQN:                  &pluginFQN,
	}
	if len(chePlugin.Containers) == 0 {
		return componentInstanceStatus, nil
	}

	podTemplate := &corev1.PodTemplateSpec{}
	componentInstanceStatus.WorkspacePodAdditions = podTemplate
	componentInstanceStatus.ExternalObjects = []runtime.Object{}

	for _, containerDef := range chePlugin.Containers {
		machineName := containerDef.Name

		var exposedPorts []int = pluginModelUtils.ExposedPortsToInts(containerDef.Ports)

		var limitOrDefault string

		if containerDef.MemoryLimit == "" {
			limitOrDefault = "128M"
		} else {
			limitOrDefault = containerDef.MemoryLimit
		}

		limit, err := resource.ParseQuantity(limitOrDefault)
		if err != nil {
			return nil, err
		}

		volumeMounts := createVolumeMounts(names, &containerDef.MountSources, []workspaceApi.Volume{}, containerDef.Volumes)

		var envVars []corev1.EnvVar
		for _, envVarDef := range containerDef.Env {
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
			Image:           containerDef.Image,
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
			VolumeMounts:             volumeMounts,
			Env:                      append(envVars, commonEnvironmentVariables(names)...),
			TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
		}
		podTemplate.Spec.Containers = append(podTemplate.Spec.Containers, container)

		for _, service := range createK8sServicesForMachines(names, machineName, exposedPorts) {
			componentInstanceStatus.ExternalObjects = append(componentInstanceStatus.ExternalObjects, &service)
		}

		for _, endpointDef := range chePlugin.Endpoints {
			attributes := map[string]string{}
			if endpointDef.Public {
				attributes["public"] = "true"
			} else {
				attributes["public"] = "false"
			}
			for name, value := range endpointDef.Attributes {
				attributes[name] = value
			}
			endpoint := workspaceApi.Endpoint {
				Name: endpointDef.Name,
				Port: int64(endpointDef.TargetPort),
				Attributes: attributes,
			}
			componentInstanceStatus.Endpoints = append(componentInstanceStatus.Endpoints, endpoint)
		}

		machineAttributes := map[string]string{}
		if limitAsInt64, canBeConverted := limit.AsInt64(); canBeConverted {
			machineAttributes[MEMORY_LIMIT_ATTRIBUTE] = strconv.FormatInt(limitAsInt64, 10)
			machineAttributes[MEMORY_REQUEST_ATTRIBUTE] = strconv.FormatInt(limitAsInt64, 10)
		}
		machineAttributes[CONTAINER_SOURCE_ATTRIBUTE] = TOOL_CONTAINER_SOURCE
		machineAttributes[PLUGIN_MACHINE_ATTRIBUTE] = chePlugin.ID

		componentInstanceStatus.Machines[machineName] = MachineDescription{
			MachineAttributes: machineAttributes,
			Ports:             exposedPorts,
		}

		for _, command := range containerDef.Commands {
			componentInstanceStatus.ContributedRuntimeCommands = append(componentInstanceStatus.ContributedRuntimeCommands,
				CheWorkspaceCommand{
					Name:        command.Name,
					CommandLine: strings.Join(command.Command, " "),
					Type:        "custom",
					Attributes: map[string]string{
						COMMAND_WORKING_DIRECTORY_ATTRIBUTE: command.WorkingDir,
						COMMAND_MACHINE_NAME_ATTRIBUTE:      machineName,
					},
				})
		}

		// TODO Manage devfile commands associated to this component ?
	}
	/*
		for _, endpointDef := range chePlugin.Endpoints {
			port := endpointDef.TargetPort

			if isTheiaOrVsCodePlugin &&
				endpointDef.Attributes != nil &&
				endpointDef.Attributes["protocol"] == "" {
				endpointDef.Attributes["protocol"] = "ws"
			}

			serverAnnotationName := func(attrName string) string {
				return join(".", "org.eclipse.che.server", attrName)
			}
			serverAnnotationAttributes := func() string {
				attrMap := map[string]string{}

				for k, v := range endpointDef.Attributes {
					if k == "protocol" {
						continue
					}
					attrMap[k] = v
				}
				attrMap["internal"] = strconv.FormatBool(!endpointDef.Public)
				res, err := json.Marshal(attrMap)
				if err != nil {
					return "{}"
				}
				return string(res)
			}

			serviceName, servicePort := serviceByPort[port].Name, servicePortName(port)
			serviceNameAndPort := join("-", serviceName, servicePort)

			ingress := extensionsv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      join("-", "ingress", names.workspaceId, endpointDef.Name),
					Namespace: names.namespace,
					Annotations: map[string]string{
						"kubernetes.io/ingress.class":                "nginx",
						"nginx.ingress.kubernetes.io/rewrite-target": "/",
						"nginx.ingress.kubernetes.io/ssl-redirect":   "false",
						"org.eclipse.che.machine.name":               join("/", cheOriginalName, containerByPort[port].Name),
						serverAnnotationName("attributes"):           serverAnnotationAttributes(),
						serverAnnotationName("port"):                 servicePortAndProtocol(port),
						serverAnnotationName("protocol"):             endpointDef.Attributes["protocol"],
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
