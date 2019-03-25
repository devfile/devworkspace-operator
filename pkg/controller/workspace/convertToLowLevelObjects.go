package workspace

import (
	"strings"

	workspacev1beta1 "github.com/che-incubator/che-workspace-crd-controller/pkg/apis/workspace/v1beta1"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/google/uuid"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type workspaceProperties struct {
	workspaceId   string
	workspaceName string
	namespace     string
	started       bool
}

func convertToCoreObjects(workspace *workspacev1beta1.Workspace) (*workspaceProperties, []runtime.Object, error) {

	uid, err := uuid.Parse(string(workspace.ObjectMeta.UID))
	if err != nil {
		return nil, nil, err
	}

	workspaceProperties := workspaceProperties{
		namespace:     workspace.Namespace,
		workspaceId:   "workspace" + strings.Join(strings.Split(uid.String(), "-")[0:3], ""),
		workspaceName: workspace.Name,
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

func buildMainDeployment(wkspProps workspaceProperties, workspace *workspacev1beta1.Workspace) (*appsv1.Deployment, error) {
	var workspaceDeploymentName = wkspProps.workspaceId + "." + cheOriginalName
	var terminationGracePeriod int64
	var replicas int32
	if wkspProps.started {
		replicas = 1
	}

	var autoMountServiceAccount = serviceAccount != ""

	fromIntOne := intstr.FromInt(1)

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
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       &fromIntOne,
					MaxUnavailable: &fromIntOne,
				},
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
					Containers: []corev1.Container{
						corev1.Container{
							Image:           "dfestal/che-rest-apis",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Name:            "che-rest-apis",
							Ports: []corev1.ContainerPort{
								corev1.ContainerPort{
									ContainerPort: 9999,
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
						},
					},
				},
			},
		},
	}
	if serviceAccount != "" {
		deploy.Spec.Template.Spec.ServiceAccountName = serviceAccount
	}

	return &deploy, nil
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

func setupTools(names workspaceProperties, tools []workspacev1beta1.ToolSpec, deployment *appsv1.Deployment) ([]runtime.Object, error) {
	var k8sObjects []runtime.Object

	pluginMetas := map[string][]model.PluginMeta{}
	workspaceEnv := map[string]string{}

	for _, tool := range tools {
		var toolType = tool.Type
		var err error
		var toolK8sObjects []runtime.Object
		switch toolType {
		case "cheEditor", "chePlugin":
			toolK8sObjects, err = setupChePlugin(names, &tool, &deployment.Spec.Template.Spec, pluginMetas, workspaceEnv)
			break
		case "kubernetes", "openshift":
			//				err = setupK8sLikeTool(&tool, deployment)
			break
		case "dockerimage":
			toolK8sObjects, err = setupDockerImageTool(names, &tool, &deployment.Spec.Template.Spec)
			break
		}
		if err != nil {
			return nil, err
		}
		k8sObjects = append(k8sObjects, toolK8sObjects...)
	}

	addWorkspaceEnvVars(&deployment.Spec.Template.Spec, workspaceEnv)

	initContainersK8sObjects, err := setupPluginInitContainers(names, &deployment.Spec.Template.Spec, pluginMetas)
	if err != nil {
		return nil, err
	}

	k8sObjects = append(k8sObjects, initContainersK8sObjects...)

	return k8sObjects, nil
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
