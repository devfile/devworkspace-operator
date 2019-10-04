package workspace

import (
	"encoding/json"
	"errors"
	"github.com/eclipse/che-plugin-broker/utils"
	"net/http"
	"strconv"
	"strings"

	workspaceApi "github.com/che-incubator/che-workspace-crd-operator/pkg/apis/workspace/v1alpha1"
	unifiedBroker "github.com/eclipse/che-plugin-broker/brokers/unified"
	vscodeBroker "github.com/eclipse/che-plugin-broker/brokers/unified/vscode"
	commonBroker "github.com/eclipse/che-plugin-broker/common"
	"github.com/eclipse/che-plugin-broker/model"
	storage "github.com/eclipse/che-plugin-broker/storage"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
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
	theStorage := storage.New()
	theCommonBroker := commonBroker.NewBroker()
	theCachingIoUtil := NewCachingIoUtil()
	theIoUtil := utils.New()
	theRand := commonBroker.NewRand()
	theHttpClient := &http.Client{}
	theUnifiedBroker := unifiedBroker.NewBrokerWithParams(theCommonBroker, theIoUtil, theStorage, theRand, theHttpClient, true)

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

	pluginMeta, err := theUnifiedBroker.GetPluginMeta(pluginFQN, controllerConfig.getPluginRegistry())
	if err != nil {
		return nil, err
	}

	isTheiaOrVsCodePlugin := false

	var chePlugin model.ChePlugin
	switch pluginMeta.Type {
	case "Che Plugin", "Che Editor":
		chePlugin = unifiedBroker.ConvertMetaToPlugin(*pluginMeta)
		break
	case "Theia plugin":
		fallthrough
	case "VS Code extension":
		broker := vscodeBroker.NewBrokerWithParams(theCommonBroker, theCachingIoUtil, theStorage, theRand, &http.Client{}, true)
		broker.ProcessPlugin(*pluginMeta)
		plugins, err := theStorage.Plugins()
		if err != nil {
			return nil, err
		}

		if len(plugins) != 1 {
			return nil, errors.New("There should be only one plugin definition for plugin " + pluginMeta.ID)
		}

		chePlugin = (plugins)[0]
		isTheiaOrVsCodePlugin = true
		break
	default:
		return nil, errors.New("Unknown plugin type: " + pluginMeta.Type)
	}

	var k8sObjects []runtime.Object

	componentInstanceStatus := &ComponentInstanceStatus {
		pluginFQN: &pluginFQN,
	}
	if len(chePlugin.Containers) == 0 {
		return componentInstanceStatus, nil
	}

	podSpec := &corev1.PodSpec{} 
	componentInstanceStatus.machineName = machineName(component)

	TODO : remplir les runtime attributes et le reste du componentInstance

	containerByPort := map[int]corev1.Container{}
	serviceByPort := map[int]corev1.Service{}

	for _, containerDef := range chePlugin.Containers {
		var containerPorts []corev1.ContainerPort
		var servicePorts []corev1.ServicePort
		for _, portDef := range containerDef.Ports {
			containerPorts = append(containerPorts, corev1.ContainerPort{
				ContainerPort: int32(portDef.ExposedPort),
				Protocol:      corev1.ProtocolTCP,
			})
			servicePorts = append(servicePorts, corev1.ServicePort{
				Name:       servicePortName(portDef.ExposedPort),
				Protocol:   servicePortProtocol,
				Port:       int32(portDef.ExposedPort),
				TargetPort: intstr.FromInt(portDef.ExposedPort),
			})
		}

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

		var volumeMounts []corev1.VolumeMount

		for _, volDef := range containerDef.Volumes {
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				MountPath: volDef.MountPath,
				Name:      "claim-che-workspace",
				SubPath:   names.workspaceId + "/" + volDef.Name + "/",
			})
		}

		var envVars []corev1.EnvVar
		for _, envVarDef := range containerDef.Env {
			envVars = append(envVars, corev1.EnvVar{
				Name:  envVarDef.Name,
				Value: envVarDef.Value,
			})
		}
		envVars = append(envVars, corev1.EnvVar{
			Name:  "CHE_MACHINE_NAME",
			Value: cheOriginalName + "/" + containerDef.Name,
		})
		container := corev1.Container{
			Name:            containerDef.Name,
			Image:           containerDef.Image,
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
			VolumeMounts:             volumeMounts,
			Env:                      append(envVars, commonEnvironmentVariables(names)...),
			TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
		}
		podSpec.Containers = append(podSpec.Containers, container)

		serviceName := join("-",
			"server",
			strings.ReplaceAll(names.workspaceId, "workspace", ""),
			cheOriginalName,
			containerDef.Name)

		if len(servicePorts) > 0 {
			service := corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serviceName,
					Namespace: names.namespace,
					Annotations: map[string]string{
						"org.eclipse.che.machine.name":   join("/", cheOriginalName, containerDef.Name),
						"org.eclipse.che.machine.source": "component",
						"org.eclipse.che.machine.plugin": strings.Split(*component.Id, ":")[0],
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
			for _, portDef := range containerDef.Ports {
				containerByPort[portDef.ExposedPort] = container
				serviceByPort[portDef.ExposedPort] = service
			}
		}
	}

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

	return k8sObjects, nil
}
