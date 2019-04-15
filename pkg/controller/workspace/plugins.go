package workspace

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	workspaceApi "github.com/che-incubator/che-workspace-crd-operator/pkg/apis/workspace/v1alpha1"
	mainBroker "github.com/eclipse/che-plugin-broker/brokers/che-plugin-broker"
	theiaBroker "github.com/eclipse/che-plugin-broker/brokers/theia"
	vscodeBroker "github.com/eclipse/che-plugin-broker/brokers/vscode"
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

func setupPluginInitContainers(names workspaceProperties, podSpec *corev1.PodSpec, pluginMetas map[string][]model.PluginMeta) ([]runtime.Object, error) {
	var k8sObjects []runtime.Object

	type initContainerDef struct {
		imageName   string
		pluginMetas []model.PluginMeta
	}

	defs := []initContainerDef{
		initContainerDef{
			imageName:   "che.workspace.plugin_broker.init.image",
			pluginMetas: []model.PluginMeta{},
		},
	}

	for brokerImageProperty, typePluginMetas := range pluginMetas {
		defs = append(defs, initContainerDef{
			imageName:   brokerImageProperty,
			pluginMetas: typePluginMetas,
		})
	}

	for _, def := range defs {
		brokerImage := workspaceConfig.getProperty(def.imageName)
		if brokerImage == nil {
			return nil, errors.New("Unknown broker docker image for : " + def.imageName)
		}

		volumes := []corev1.VolumeMount{
			corev1.VolumeMount{
				MountPath: "/plugins/",
				Name:      "claim-che-workspace",
				SubPath:   names.workspaceId + "/plugins/",
			},
		}

		key := commonBroker.NewRand().String(6)
		containerName := key + "broker"
		args := []string{
			"-disable-push",
			"-runtime-id",
			join(":",
				names.workspaceId,
				"",
				"anonymous",
			),
		}

		if len(def.pluginMetas) > 0 {
			configMapName := key + "broker-config-map"
			configMapVolume := key + "broker-config-volume"
			configMapContent, err := json.MarshalIndent(def.pluginMetas, "", "")
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

			volumes = append(volumes, corev1.VolumeMount{
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

			ImagePullPolicy: corev1.PullIfNotPresent,
			VolumeMounts:    volumes,
		})
	}
	return k8sObjects, nil
}

func setupChePlugin(names workspaceProperties, component *workspaceApi.ComponentSpec, podSpec *corev1.PodSpec, pluginMetas map[string][]model.PluginMeta, workspaceEnv map[string]string) ([]runtime.Object, error) {
	pluginMeta, err := getPluginMeta(workspaceConfig.getPluginRegistry(), *component.Id)
	if err != nil {
		return nil, err
	}

	var processPlugin func(meta model.PluginMeta) error
	theStorage := storage.New()
	theCommonBroker := commonBroker.NewBroker()
	theIoUtil := NewCachingIoUtil()
	theRand := commonBroker.NewRand()

	isTheiaOrVsCodePlugin := false

	var brokerImageProperty string

	switch pluginMeta.Type {
	case "Che Plugin", "Che Editor":
		broker := mainBroker.NewBrokerWithParams(theCommonBroker, theIoUtil, theStorage)
		processPlugin = broker.ProcessPlugin
		brokerImageProperty = "che.workspace.plugin_broker.image"
		break
	case "Theia plugin":
		broker := theiaBroker.NewBrokerWithParams(theCommonBroker, theIoUtil, theStorage, theRand)
		processPlugin = broker.ProcessPlugin
		brokerImageProperty = "che.workspace.plugin_broker.theia.image"
		isTheiaOrVsCodePlugin = true
		break
	case "VS Code extension":
		broker := vscodeBroker.NewBrokerWithParams(theCommonBroker, theIoUtil, theStorage, theRand, &http.Client{})
		processPlugin = broker.ProcessPlugin
		brokerImageProperty = "che.workspace.plugin_broker.vscode.image"
		isTheiaOrVsCodePlugin = true
		break
	default:
		return nil, errors.New("Unknown plugin type: " + pluginMeta.Type)
	}

	typePluginMetas, isThere := pluginMetas[brokerImageProperty]
	if !isThere {
		typePluginMetas = []model.PluginMeta{}
	}
	newTypePluginMetas := append(typePluginMetas, *pluginMeta)
	pluginMetas[brokerImageProperty] = newTypePluginMetas

	err = processPlugin(*pluginMeta)
	if err != nil {
		return nil, err
	}

	plugins, err := theStorage.Plugins()
	if err != nil {
		return nil, err
	}

	if len(*plugins) != 1 {
		return nil, errors.New("There should be only one plugin definition for plugin " + pluginMeta.ID)
	}

	componentingConf := (*plugins)[0]

	for _, envVar := range componentingConf.WorkspaceEnv {
		workspaceEnv[envVar.Name] = envVar.Value
	}

	var k8sObjects []runtime.Object

	containerByPort := map[int]corev1.Container{}
	serviceByPort := map[int]corev1.Service{}

	for _, containerDef := range componentingConf.Containers {

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
			VolumeMounts: volumeMounts,
			Env:          append(envVars, commonEnvironmentVariables(names)...),
		}
		podSpec.Containers = append(podSpec.Containers, container)

		var serviceName string
		if isTheiaOrVsCodePlugin &&
			len(containerDef.Ports) == 1 {
			for _, endpointDef := range componentingConf.Endpoints {
				if endpointDef.TargetPort == containerDef.Ports[0].ExposedPort {
					serviceName = endpointDef.Name
					break
				}
			}
		}

		if serviceName == "" {
			serviceName = join("-",
				"server",
				strings.ReplaceAll(names.workspaceId, "workspace", ""),
				cheOriginalName,
				containerDef.Name)
		}

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

	for _, endpointDef := range componentingConf.Endpoints {
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
						Host: join(".", serviceNameAndPort, workspaceConfig.getIngressGlobalDomain()),
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
		k8sObjects = append(k8sObjects, &ingress)
	}

	return k8sObjects, nil
}

func addWorkspaceEnvVars(podSpec *corev1.PodSpec, workspaceEnv map[string]string) {
	newEnvs := []corev1.EnvVar{}
	for key, value := range workspaceEnv {
		newEnvs = append(newEnvs, corev1.EnvVar{
			Name:  key,
			Value: value,
		})
	}

	for index := range podSpec.Containers {
		for _, envVar := range podSpec.Containers[index].Env {
			if envVar.Name == "THEIA_PLUGINS" {
				newContainerEnvs := append(podSpec.Containers[index].Env, newEnvs...)
				podSpec.Containers[index].Env = newContainerEnvs
				break
			}
		}
	}
}
