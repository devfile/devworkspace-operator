package workspace

import (
	"strings"

	workspaceApi "github.com/che-incubator/che-workspace-crd-operator/pkg/apis/workspace/v1alpha1"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/google/uuid"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type workspaceProperties struct {
	workspaceId    string
	workspaceName  string
	namespace      string
	started        bool
	cheApiExternal string
}

func convertToCoreObjects(workspace *workspaceApi.Workspace) (*workspaceProperties, []runtime.Object, error) {

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

	cheRestApisK8sObjects, externalUrl, err := addCheRestApis(workspaceProperties, &mainDeployment.Spec.Template.Spec)
	if err != nil {
		return &workspaceProperties, nil, err
	}
	workspaceProperties.cheApiExternal = externalUrl

	k8sComponentsObjects, err := setupComponents(workspaceProperties, workspace.Spec.DevFile.Components, mainDeployment)
	if err != nil {
		return &workspaceProperties, nil, err
	}
	k8sComponentsObjects = append(k8sComponentsObjects, cheRestApisK8sObjects...)

	return &workspaceProperties, append(k8sComponentsObjects, mainDeployment), nil
}

func buildMainDeployment(wkspProps workspaceProperties, workspace *workspaceApi.Workspace) (*appsv1.Deployment, error) {
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
						"deployment":         workspaceDeploymentName,
						"che.original_name":  cheOriginalName,
						"che.workspace_id":   wkspProps.workspaceId,
						"che.workspace_name": wkspProps.workspaceName,
					},
					Name: workspaceDeploymentName,
				},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken:  &autoMountServiceAccount,
					RestartPolicy:                 "Always",
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					Containers:                    []corev1.Container{},
				},
			},
		},
	}
	if serviceAccount != "" {
		deploy.Spec.Template.Spec.ServiceAccountName = serviceAccount
	}

	return &deploy, nil
}

func setupPersistentVolumeClaim(workspace *workspaceApi.Workspace, deployment *appsv1.Deployment) error {
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



func setupComponents(names workspaceProperties, components []workspaceApi.ComponentSpec, deployment *appsv1.Deployment) ([]runtime.Object, error) {
	var k8sObjects []runtime.Object

	pluginFQNs := []model.PluginFQN{}
	
	workspacePodAdditions := []corev1.PodSpec{}

	for _, component := range components {
		var componentType = component.Type
		var err error
		var componentInstanceStatus *ComponentInstanceStatus
		switch componentType {
		case "cheEditor", "chePlugin":
			componentInstanceStatus, err = setupChePlugin(names, &component)
			if componentInstanceStatus.pluginFQN != nil {
				pluginFQNs = append(pluginFQNs, *componentInstanceStatus.pluginFQN)
			}
			break
		case "kubernetes", "openshift":
			componentInstanceStatus, err = setupK8sLikeComponent(names, &component, &deployment.Spec.Template.Spec)
			break
		case "dockerimage":
			componentInstanceStatus, err = setupDockerImageComponent(names, &component, &deployment.Spec.Template.Spec)
			break
		}
		if err != nil {
			return nil, err
		}
		k8sObjects = append(k8sObjects, componentInstanceStatus.externalObjects...)
		if componentInstanceStatus.WorkspacePodAdditions != nil {
			workspacePodAdditions = append(workspacePodAdditions, *componentInstanceStatus.WorkspacePodAdditions)
		}
	}

	// Also fixes service labels
	mergeWorkspaceAdditions(&deployment.Spec.Template.Spec, k8sObjects, workspacePodAdditions)
	
	precreateSubpathsInitContainer(names, &deployment.Spec.Template.Spec)
	initContainersK8sObjects, err := setupPluginInitContainers(names, &deployment.Spec.Template.Spec, pluginFQNs)
	if err != nil {
		return nil, err
	}
	
	k8sObjects = append(k8sObjects, initContainersK8sObjects...)

	TODO merge the endpoints of components per machine, et create the WorkspaceExposer CR

	TODO call the method on the WorkspaceExposer that returns the ingresses in k8sObjects

	return k8sObjects, nil
}

func precreateSubpathsInitContainer(names workspaceProperties, podSpec *corev1.PodSpec) {
	podSpec.InitContainers = append(podSpec.InitContainers, corev1.Container{
		Name:  "precreate-subpaths",
		Image: "registry.access.redhat.com/ubi8/ubi-minimal",
		Command: []string{ "/usr/bin/mkdir" },
		Args: []string{
			"-p",
			"-v",
			"-m",
			"777",
			"/tmp/che-workspaces/" + names.workspaceId,
		},
		ImagePullPolicy: corev1.PullIfNotPresent,
		VolumeMounts: []corev1.VolumeMount{
			corev1.VolumeMount{
				MountPath: "/tmp/che-workspaces",
				Name:      "claim-che-workspace",
				ReadOnly:  false,
			},
		},
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
	})
}
