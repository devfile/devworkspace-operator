//
// Copyright (c) 2019-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package component

import (
	"errors"
	"strings"

	modelutils "github.com/che-incubator/che-workspace-operator/pkg/controller/modelutils/k8s"
	"github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/server"

	workspaceApi "github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	. "github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/config"
	. "github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/model"
	"github.com/google/uuid"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func ConvertToCoreObjects(workspace *workspaceApi.Workspace) (*WorkspaceContext, *workspaceApi.WorkspaceRouting, []ComponentInstanceStatus, []runtime.Object, error) {

	uid, err := uuid.Parse(string(workspace.ObjectMeta.UID))
	if err != nil {
		return nil, nil, nil, nil, err
	}

	wkspCtx := WorkspaceContext{
		Namespace:     workspace.Namespace,
		WorkspaceId:   "workspace" + strings.Join(strings.Split(uid.String(), "-")[0:3], ""),
		WorkspaceName: workspace.Name,
		Started:       workspace.Spec.Started,
		RoutingClass:  workspace.Spec.RoutingClass,
		Creator:       workspace.Annotations[WorkspaceCreatorAnnotation],
	}

	if !wkspCtx.Started {
		return &wkspCtx, &workspaceApi.WorkspaceRouting{
			ObjectMeta: metav1.ObjectMeta{
				Name:      wkspCtx.WorkspaceId,
				Namespace: wkspCtx.Namespace,
			},
			Spec: workspaceApi.WorkspaceRoutingSpec{
				Exposed:             wkspCtx.Started,
				RoutingClass:        wkspCtx.RoutingClass,
				IngressGlobalDomain: ControllerCfg.GetIngressGlobalDomain(),
				WorkspacePodSelector: map[string]string{
					CheOriginalNameLabel: CheOriginalName,
					WorkspaceIDLabel:     wkspCtx.WorkspaceId,
				},
				Services: map[string]workspaceApi.ServiceDescription{},
			},
		}, nil, []runtime.Object{}, nil
	}

	mainDeployment, err := buildMainDeployment(wkspCtx, workspace)
	if err != nil {
		return &wkspCtx, nil, nil, nil, err
	}

	err = setupPersistentVolumeClaim(workspace, mainDeployment)
	if err != nil {
		return &wkspCtx, nil, nil, nil, err
	}

	cheRestApisK8sObjects, externalUrl, err := server.AddCheRestApis(wkspCtx, &mainDeployment.Spec.Template.Spec)
	if err != nil {
		return &wkspCtx, nil, nil, nil, err
	}
	wkspCtx.CheApiExternal = externalUrl

	workspaceRouting, componentStatuses, k8sComponentsObjects, err := setupComponents(wkspCtx, workspace.Spec.Devfile, mainDeployment)
	if err != nil {
		return &wkspCtx, nil, nil, nil, err
	}
	k8sComponentsObjects = append(k8sComponentsObjects, cheRestApisK8sObjects...)

	return &wkspCtx, workspaceRouting, componentStatuses, append(k8sComponentsObjects, mainDeployment), nil
}

func buildMainDeployment(wkspCtx WorkspaceContext, workspace *workspaceApi.Workspace) (*appsv1.Deployment, error) {
	var workspaceDeploymentName = wkspCtx.WorkspaceId + "." + CheOriginalName
	var terminationGracePeriod int64
	var replicas int32
	if wkspCtx.Started {
		replicas = 1
	}

	var autoMountServiceAccount = ServiceAccount != ""

	fromIntOne := intstr.FromInt(1)

	var user *int64
	if !ControllerCfg.IsOpenShift() {
		uID := int64(1234)
		user = &uID
	}

	deploy := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workspaceDeploymentName,
			Namespace: workspace.Namespace,
			Labels: map[string]string{
				WorkspaceIDLabel: wkspCtx.WorkspaceId,
			},
			Annotations: map[string]string{
				WorkspaceCreatorAnnotation: wkspCtx.Creator,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"deployment":         workspaceDeploymentName,
					CheOriginalNameLabel: CheOriginalName,
					WorkspaceIDLabel:     wkspCtx.WorkspaceId,
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
						CheOriginalNameLabel: CheOriginalName,
						WorkspaceIDLabel:     wkspCtx.WorkspaceId,
						WorkspaceNameLabel:   wkspCtx.WorkspaceName,
					},
					Annotations: map[string]string{
						WorkspaceCreatorAnnotation: wkspCtx.Creator,
					},
					Name: workspaceDeploymentName,
				},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken:  &autoMountServiceAccount,
					RestartPolicy:                 "Always",
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					Containers:                    []corev1.Container{},
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser: user,
						FSGroup:   user,
					},
				},
			},
		},
	}
	if ServiceAccount != "" {
		deploy.Spec.Template.Spec.ServiceAccountName = ServiceAccount
	}

	return &deploy, nil
}

func setupPersistentVolumeClaim(workspace *workspaceApi.Workspace, deployment *appsv1.Deployment) error {
	var workspaceClaim = corev1.PersistentVolumeClaimVolumeSource{
		ClaimName: ControllerCfg.GetWorkspacePVCName(),
	}
	deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: ControllerCfg.GetWorkspacePVCName(),
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &workspaceClaim,
			},
		},
	}
	return nil
}

func setupComponents(wkspCtx WorkspaceContext, devfile workspaceApi.DevfileSpec, deployment *appsv1.Deployment) (*workspaceApi.WorkspaceRouting, []ComponentInstanceStatus, []runtime.Object, error) {
	components := devfile.Components
	var k8sObjects []runtime.Object

	var pluginComponents []workspaceApi.ComponentSpec

	componentInstanceStatuses := []ComponentInstanceStatus{}

	for _, component := range components {
		var componentType = component.Type
		var err error
		var componentInstanceStatus *ComponentInstanceStatus
		switch componentType {
		case workspaceApi.CheEditor, workspaceApi.ChePlugin:
			pluginComponents = append(pluginComponents, component)
			continue
		case workspaceApi.Kubernetes, workspaceApi.Openshift:
			componentInstanceStatus, err = setupK8sLikeComponent(wkspCtx, &component)
			break
		case workspaceApi.Dockerimage:
			componentInstanceStatus, err = setupDockerimageComponent(wkspCtx, devfile.Commands, &component)
			break
		}
		if err != nil {
			return nil, nil, nil, err
		}
		k8sObjects = append(k8sObjects, componentInstanceStatus.ExternalObjects...)
		componentInstanceStatuses = append(componentInstanceStatuses, *componentInstanceStatus)
	}
	pluginComponentStatuses, err := setupChePlugins(wkspCtx, pluginComponents)
	if err != nil {
		return nil, nil, nil, err
	}

	componentInstanceStatuses = append(componentInstanceStatuses, pluginComponentStatuses...)
	for _, cis := range componentInstanceStatuses {
		k8sObjects = append(k8sObjects, cis.ExternalObjects...)
	}

	err = mergeWorkspaceAdditions(deployment, componentInstanceStatuses, k8sObjects)
	if err != nil {
		return nil, nil, nil, err
	}

	precreateSubpathsInitContainer(wkspCtx, &deployment.Spec.Template.Spec)

	workspaceRouting := buildWorkspaceRouting(wkspCtx, componentInstanceStatuses)

	// TODO store the annotation of the workspaceAPi: with the defer ????

	return workspaceRouting, componentInstanceStatuses, k8sObjects, nil
}

func buildWorkspaceRouting(wkspCtx WorkspaceContext, componentInstanceStatuses []ComponentInstanceStatus) *workspaceApi.WorkspaceRouting {
	services := map[string]workspaceApi.ServiceDescription{}
	for _, componentInstanceStatus := range componentInstanceStatuses {
		for containerName, container := range componentInstanceStatus.Containers {
			containerEndpoints := []workspaceApi.Endpoint{}
			for _, port := range container.Ports {
				port64 := int64(port)
				for _, endpoint := range componentInstanceStatus.Endpoints {
					if endpoint.Port != port64 {
						continue
					}
					if endpoint.Attributes == nil {
						endpoint.Attributes = map[workspaceApi.EndpointAttribute]string{}
					}
					// public is the default.
					if _, exists := endpoint.Attributes[workspaceApi.PUBLIC_ENDPOINT_ATTRIBUTE]; !exists {
						endpoint.Attributes[workspaceApi.PUBLIC_ENDPOINT_ATTRIBUTE] = "true"
					}
					containerEndpoints = append(containerEndpoints, endpoint)
				}
			}
			if len(containerEndpoints) > 0 {
				services[containerName] = workspaceApi.ServiceDescription{
					ServiceName: modelutils.ContainerServiceName(wkspCtx.WorkspaceId, containerName),
					Endpoints:   containerEndpoints,
				}
			}
		}
	}
	return &workspaceApi.WorkspaceRouting{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wkspCtx.WorkspaceId,
			Namespace: wkspCtx.Namespace,
		},
		Spec: workspaceApi.WorkspaceRoutingSpec{
			Exposed:             wkspCtx.Started,
			RoutingClass:        wkspCtx.RoutingClass,
			IngressGlobalDomain: ControllerCfg.GetIngressGlobalDomain(),
			WorkspacePodSelector: map[string]string{
				CheOriginalNameLabel: CheOriginalName,
				WorkspaceIDLabel:     wkspCtx.WorkspaceId,
			},
			Services: services,
		},
	}
}

//TODO Think of the admission controller to add the name of the user in the workspace?
// In any case add the name of the users in the custom resource of the workspace. + the workspace routing class.
func precreateSubpathsInitContainer(wkspCtx WorkspaceContext, podSpec *corev1.PodSpec) {
	podSpec.InitContainers = append(podSpec.InitContainers, corev1.Container{
		Name:    "precreate-subpaths",
		Image:   "registry.access.redhat.com/ubi8/ubi-minimal",
		Command: []string{"/usr/bin/mkdir"},
		Args: []string{
			"-p",
			"-v",
			"-m",
			"777",
			"/tmp/che-workspaces/" + wkspCtx.WorkspaceId,
		},
		ImagePullPolicy: corev1.PullPolicy(ControllerCfg.GetSidecarPullPolicy()),
		VolumeMounts: []corev1.VolumeMount{
			{
				MountPath: "/tmp/che-workspaces",
				Name:      ControllerCfg.GetWorkspacePVCName(),
				ReadOnly:  false,
			},
		},
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
	})
}

func mergeWorkspaceAdditions(workspaceDeployment *appsv1.Deployment, componentInstanceStatuses []ComponentInstanceStatus, k8sObjects []runtime.Object) error {
	workspacePodAdditions := []corev1.PodTemplateSpec{}
	for _, componentInstanceStatus := range componentInstanceStatuses {
		if componentInstanceStatus.WorkspacePodAdditions != nil {
			workspacePodAdditions = append(workspacePodAdditions, *componentInstanceStatus.WorkspacePodAdditions)
		}
	}
	workspacePodTemplate := &workspaceDeployment.Spec.Template

	// "Set"s to store k8s object names and detect duplicates
	containerNames := map[string]bool{}
	initContainerNames := map[string]bool{}
	volumeNames := map[string]bool{}
	pullSecretNames := map[string]bool{}

	for _, addition := range workspacePodAdditions {
		for annotKey, annotValue := range addition.Annotations {
			workspacePodTemplate.Annotations[annotKey] = annotValue
		}

		for labelKey, labelValue := range addition.Labels {
			workspacePodTemplate.Labels[labelKey] = labelValue
		}

		for _, container := range addition.Spec.Containers {
			if containerNames[container.Name] {
				return errors.New("Duplicate containers in the workspace definition: " + container.Name)
			}
			containerNames[container.Name] = true
			workspacePodTemplate.Spec.Containers = append(workspacePodTemplate.Spec.Containers, container)
		}

		for _, container := range addition.Spec.InitContainers {
			if initContainerNames[container.Name] {
				return errors.New("Duplicate init conainers in the workspace definition: " + container.Name)
			}
			initContainerNames[container.Name] = true
			workspacePodTemplate.Spec.InitContainers = append(workspacePodTemplate.Spec.InitContainers, container)
		}

		for _, volume := range addition.Spec.Volumes {
			if volumeNames[volume.Name] {
				return errors.New("Duplicate volumes in the workspace definition: " + volume.Name)
			}
			volumeNames[volume.Name] = true
			workspacePodTemplate.Spec.Volumes = append(workspacePodTemplate.Spec.Volumes, volume)
		}

		for _, pullSecret := range addition.Spec.ImagePullSecrets {
			if pullSecretNames[pullSecret.Name] {
				continue
			}
			pullSecretNames[pullSecret.Name] = true
			workspacePodTemplate.Spec.ImagePullSecrets = append(workspacePodTemplate.Spec.ImagePullSecrets, pullSecret)
		}
	}
	workspacePodTemplate.Labels[server.DEPLOYMENT_NAME_LABEL] = workspaceDeployment.Name
	for _, externalObject := range k8sObjects {
		service, isAService := externalObject.(*corev1.Service)
		if isAService {
			service.Spec.Selector[server.DEPLOYMENT_NAME_LABEL] = workspaceDeployment.Name
		}
	}
	return nil
}
