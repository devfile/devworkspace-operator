package workspace

import (
	"encoding/json"
	"strconv"
	"strings"

	workspacev1beta1 "github.com/che-incubator/che-workspace-crd-controller/pkg/apis/workspace/v1beta1"
	"github.com/google/uuid"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var (
	defaultImagePullPolicy    = corev1.PullAlways
	defaultApiEndpoint        = "http://che-che.192.168.42.174.nip.io/api"
	cheOriginalName           = "che-wksp"
	authEnabled               = "false"
	servicePortProtocol       = corev1.ProtocolTCP
	serviceAccount            = ""
	sidecarDefaultMemoryLimit = "128M"
	pvcStorageSize            = "1Gi"
)

type workspaceProperties struct {
	workspaceId   string
	workspaceName string
	namespace     string
	started       bool
}

func join(sep string, parts ...string) string {
	return strings.Join(parts, sep)
}

func convertToCoreObjects(workspace *workspacev1beta1.Workspace) (*workspaceProperties, []runtime.Object, error) {

	uid, err := uuid.Parse(string(workspace.ObjectMeta.UID))
	if err != nil {
		return nil, nil, err
	}

	workspaceProperties := workspaceProperties{
		namespace:     workspace.Namespace,
		workspaceId:   "workspace" + strings.Join(strings.Split(uid.String(), "-")[0:3], ""),
		workspaceName: workspace.Spec.DevFile.Name,
		started:       workspace.Spec.Started,
	}

	if !workspaceProperties.started {
		return &workspaceProperties, []runtime.Object{}, nil
	}

	mainDeployment, err := buildMainDeployment(workspaceProperties, workspace)
	if err != nil {
		return &workspaceProperties, nil, err
	}
	err = setupPersistentVolumeClaim(workspace, mainDeployment)
	if err != nil {
		return &workspaceProperties, nil, err
	}
	k8sToolsObjects, err := setupTools(workspaceProperties, workspace.Spec.DevFile.Tools, mainDeployment)
	if err != nil {
		return &workspaceProperties, nil, err
	}

	return &workspaceProperties, append(k8sToolsObjects, mainDeployment), nil
}

func setupPersistentVolumeClaim(workspace *workspacev1beta1.Workspace, deployment *appsv1.Deployment) error {
	var workspaceClaim = corev1.PersistentVolumeClaimVolumeSource{
		ClaimName: "claim-che-workspace",
	}
	deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
		corev1.Volume{
			Name: "claim-che-workspace",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &workspaceClaim,
			},
		},
	}
	return nil
}

func commonEnvironmentVariables(names workspaceProperties) []corev1.EnvVar {
	return []corev1.EnvVar{
		corev1.EnvVar{
			Name: "CHE_MACHINE_TOKEN",
		},
		corev1.EnvVar{
			Name:  "CHE_PROJECTS_ROOT",
			Value: "/projects",
		},
		corev1.EnvVar{
			Name:  "CHE_API",
			Value: defaultApiEndpoint,
		},
		corev1.EnvVar{
			Name:  "CHE_API_INTERNAL",
			Value: defaultApiEndpoint,
		},
		corev1.EnvVar{
			Name:  "CHE_API_EXTERNAL",
			Value: defaultApiEndpoint,
		},
		corev1.EnvVar{
			Name:  "CHE_WORKSPACE_NAME",
			Value: names.workspaceName,
		},
		corev1.EnvVar{
			Name:  "CHE_WORKSPACE_ID",
			Value: names.workspaceId,
		},
		corev1.EnvVar{
			Name:  "CHE_AUTH_ENABLED",
			Value: authEnabled,
		},
		corev1.EnvVar{
			Name:  "CHE_WORKSPACE_NAMESPACE",
			Value: names.namespace,
		},
	}
}

func setupChePlugin(names workspaceProperties, tool *workspacev1beta1.ToolSpec, podSpec *corev1.PodSpec) ([]runtime.Object, error) {
	pluginMeta, err := getPluginMeta(workspaceConfig.getPluginRegistry(), *tool.Id)
	if err != nil {
		return nil, err
	}
	toolingConf, err := ProcessPlugin(pluginMeta)
	if err != nil {
		return nil, err
	}

	var k8sObjects []runtime.Object

	containerByPort := map[int]corev1.Container{}
	serviceByPort := map[int]corev1.Service{}

	portAsString := func(port int) string {
		return strconv.FormatInt(int64(port), 10)
	}

	servicePortName := func(port int) string {
		return "srv-" + portAsString(port)
	}
	servicePortAndProtocol := func(port int) string {
		return join("/", portAsString(port), strings.ToLower(string(servicePortProtocol)))
	}

	for _, containerDef := range toolingConf.Containers {

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

		log.Info("limit for container", "container", containerDef.Name, "limit", containerDef.MemoryLimit)
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

		service := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: join("-",
					"server",
					strings.ReplaceAll(names.workspaceId, "workspace", ""),
					cheOriginalName,
					containerDef.Name),
				Namespace: names.namespace,
				Annotations: map[string]string{
					"org.eclipse.che.machine.name": join("/", cheOriginalName, containerDef.Name),
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

	for _, endpointDef := range toolingConf.Endpoints {
		port := endpointDef.TargetPort

		serverAnnotationName := func(attrName string) string {
			return join(".", "org.eclipse.che.server", endpointDef.Name, "attributes")
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

func setupTools(names workspaceProperties, tools []workspacev1beta1.ToolSpec, deployment *appsv1.Deployment) ([]runtime.Object, error) {
	var k8sObjects []runtime.Object

	for _, tool := range tools {
		var toolType = tool.Type
		var err error
		var toolK8sObjects []runtime.Object
		switch toolType {
		case "cheEditor", "chePlugin":
			toolK8sObjects, err = setupChePlugin(names, &tool, &deployment.Spec.Template.Spec)
			break
		case "kubernetes", "openshift":
			//				err = setupK8sLikeTool(&tool, deployment)
			break
		case "dockerImage":
			//				err = setupDockerImageTool(&tool, deployment)
			break
		}
		if err != nil {
			return nil, err
		}
		k8sObjects = append(k8sObjects, toolK8sObjects...)
	}

	return k8sObjects, nil
}

func buildMainDeployment(wkspProps workspaceProperties, workspace *workspacev1beta1.Workspace) (*appsv1.Deployment, error) {
	var workspaceDeploymentName = wkspProps.workspaceId + "." + cheOriginalName
	var terminationGracePeriod int64
	var replicas int32
	if wkspProps.started {
		replicas = 1
	}

	var autoMountServiceAccount = serviceAccount != ""

	deploy := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workspaceDeploymentName,
			Namespace: workspace.Namespace,
			Labels: map[string]string{
				"che.workspace_id": wkspProps.workspaceId,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"deployment":        workspaceDeploymentName,
					"che.original_name": cheOriginalName,
					"che.workspace_id":  wkspProps.workspaceId,
				},
			},
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: "RollingUpdate",
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"deployment":        workspaceDeploymentName,
						"che.original_name": cheOriginalName,
						"che.workspace_id":  wkspProps.workspaceId,
					},
					Name: workspaceDeploymentName,
				},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken:  &autoMountServiceAccount,
					RestartPolicy:                 "Always",
					TerminationGracePeriodSeconds: &terminationGracePeriod,
				},
			},
		},
	}
	if serviceAccount != "" {
		deploy.Spec.Template.Spec.ServiceAccountName = serviceAccount
	}

	return &deploy, nil
}
