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

	workspaceApi "github.com/che-incubator/che-workspace-crd-operator/pkg/apis/workspace/v1alpha1"
	. "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/config"
	. "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/model"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/google/uuid"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func ConvertToCoreObjects(workspace *workspaceApi.Workspace) (*WorkspaceProperties, *workspaceApi.WorkspaceExposure, []ComponentInstanceStatus, []runtime.Object, error) {

	uid, err := uuid.Parse(string(workspace.ObjectMeta.UID))
	if err != nil {
		return nil, nil, nil, nil, err
	}

	workspaceProperties := WorkspaceProperties{
		Namespace:     workspace.Namespace,
		WorkspaceId:   "workspace" + strings.Join(strings.Split(uid.String(), "-")[0:3], ""),
		WorkspaceName: workspace.Name,
		Started:       workspace.Spec.Started,
		ExposureClass: workspace.Spec.ExposureClass,
	}

	if !workspaceProperties.Started {
		return &workspaceProperties, &workspaceApi.WorkspaceExposure{
			ObjectMeta: metav1.ObjectMeta{
				Name:      workspaceProperties.WorkspaceId,
				Namespace: workspaceProperties.Namespace,
			},
			Spec: workspaceApi.WorkspaceExposureSpec{
				Exposed:             workspaceProperties.Started,
				ExposureClass:       workspaceProperties.ExposureClass,
				IngressGlobalDomain: ControllerCfg.GetIngressGlobalDomain(),
				WorkspacePodSelector: map[string]string{
					"che.original_name": CheOriginalName,
					"che.workspace_id":  workspaceProperties.WorkspaceId,
				},
				Services: map[string]workspaceApi.ServiceDescription{},
			},
		}, nil, []runtime.Object{}, nil
	}

	mainDeployment, err := buildMainDeployment(workspaceProperties, workspace)
	if err != nil {
		return &workspaceProperties, nil, nil, nil, err
	}

	err = setupPersistentVolumeClaim(workspace, mainDeployment)
	if err != nil {
		return &workspaceProperties, nil, nil, nil, err
	}

	cheRestApisK8sObjects, externalUrl, err := addCheRestApis(workspaceProperties, &mainDeployment.Spec.Template.Spec)
	if err != nil {
		return &workspaceProperties, nil, nil, nil, err
	}
	workspaceProperties.CheApiExternal = externalUrl

	workspaceExposure, componentStatuses, k8sComponentsObjects, err := setupComponents(workspaceProperties, workspace.Spec.Devfile, mainDeployment)
	if err != nil {
		return &workspaceProperties, nil, nil, nil, err
	}
	k8sComponentsObjects = append(k8sComponentsObjects, cheRestApisK8sObjects...)

	return &workspaceProperties, workspaceExposure, componentStatuses, append(k8sComponentsObjects, mainDeployment), nil
}

func buildMainDeployment(wkspProps WorkspaceProperties, workspace *workspaceApi.Workspace) (*appsv1.Deployment, error) {
	var workspaceDeploymentName = wkspProps.WorkspaceId + "." + CheOriginalName
	var terminationGracePeriod int64
	var replicas int32
	if wkspProps.Started {
		replicas = 1
	}

	var autoMountServiceAccount = ServiceAccount != ""

	fromIntOne := intstr.FromInt(1)

	deploy := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workspaceDeploymentName,
			Namespace: workspace.Namespace,
			Labels: map[string]string{
				"che.workspace_id": wkspProps.WorkspaceId,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"deployment":        workspaceDeploymentName,
					"che.original_name": CheOriginalName,
					"che.workspace_id":  wkspProps.WorkspaceId,
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
						"che.original_name":  CheOriginalName,
						"che.workspace_id":   wkspProps.WorkspaceId,
						"che.workspace_name": wkspProps.WorkspaceName,
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
	if ServiceAccount != "" {
		deploy.Spec.Template.Spec.ServiceAccountName = ServiceAccount
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

func setupComponents(names WorkspaceProperties, devfile workspaceApi.DevFileSpec, deployment *appsv1.Deployment) (*workspaceApi.WorkspaceExposure, []ComponentInstanceStatus, []runtime.Object, error) {
	components := devfile.Components
	k8sObjects := []runtime.Object{}

	pluginFQNs := []model.PluginFQN{}

	componentInstanceStatuses := []ComponentInstanceStatus{}

	for _, component := range components {
		var componentType = component.Type
		var err error
		var componentInstanceStatus *ComponentInstanceStatus
		switch componentType {
		case "cheEditor", "chePlugin":
			componentInstanceStatus, err = setupChePlugin(names, &component)
			if err != nil {
				return nil, nil, nil, err
			}
			if componentInstanceStatus.PluginFQN != nil {
				pluginFQNs = append(pluginFQNs, *componentInstanceStatus.PluginFQN)
			}
			break
		case "kubernetes", "openshift":
			componentInstanceStatus, err = setupK8sLikeComponent(names, &component)
			break
		case "dockerimage":
			componentInstanceStatus, err = setupDockerimageComponent(names, devfile.Commands, &component)
			break
		}
		if err != nil {
			return nil, nil, nil, err
		}
		k8sObjects = append(k8sObjects, componentInstanceStatus.ExternalObjects...)
		componentInstanceStatuses = append(componentInstanceStatuses, *componentInstanceStatus)
	}

	mergeWorkspaceAdditions(deployment, componentInstanceStatuses, k8sObjects)

	precreateSubpathsInitContainer(names, &deployment.Spec.Template.Spec)
	initContainersK8sObjects, err := setupPluginInitContainers(names, &deployment.Spec.Template.Spec, pluginFQNs)
	if err != nil {
		return nil, nil, nil, err
	}

	k8sObjects = append(k8sObjects, initContainersK8sObjects...)

	workspaceExposure := buildWorkspaceExposure(names, componentInstanceStatuses)

	// TODO store the annotation of the workspaceAPi: avec le defer ????

	return workspaceExposure, componentInstanceStatuses, k8sObjects, nil
}

func buildWorkspaceExposure(wkspProperties WorkspaceProperties, componentInstanceStatuses []ComponentInstanceStatus) *workspaceApi.WorkspaceExposure {
	services := map[string]workspaceApi.ServiceDescription{}
	for _, componentInstanceStatus := range componentInstanceStatuses {
		for machineName, machine := range componentInstanceStatus.Machines {
			machineEndpoints := []workspaceApi.Endpoint{}
			for _, port := range machine.Ports {
				port64 := int64(port)
				for _, endpoint := range componentInstanceStatus.Endpoints {
					if endpoint.Port != port64 {
						continue
					}
					if endpoint.Attributes == nil {
						endpoint.Attributes = map[string]string{}
					}
					// public is the default.
					if _, exists := endpoint.Attributes["public"]; !exists {
						endpoint.Attributes["public"] = "true"
					}
					machineEndpoints = append(machineEndpoints, endpoint)
				}
			}
			if len(machineEndpoints) > 0 {
				services[machineName] = workspaceApi.ServiceDescription{
					ServiceName: machineServiceName(wkspProperties, machineName),
					Endpoints:   machineEndpoints,
				}
			}
		}
	}
	return &workspaceApi.WorkspaceExposure{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wkspProperties.WorkspaceId,
			Namespace: wkspProperties.Namespace,
		},
		Spec: workspaceApi.WorkspaceExposureSpec{
			Exposed:             wkspProperties.Started,
			ExposureClass:       wkspProperties.ExposureClass,
			IngressGlobalDomain: ControllerCfg.GetIngressGlobalDomain(),
			WorkspacePodSelector: map[string]string{
				"che.original_name": CheOriginalName,
				"che.workspace_id":  wkspProperties.WorkspaceId,
			},
			Services: services,
		},
	}
}

// Penser au admission controller pour ajouter le nom du user dnas le workspace ? E tout cas ajouter le nom du
// users dans la custom resource du workspace. + la classe de workspace exposure.

func precreateSubpathsInitContainer(names WorkspaceProperties, podSpec *corev1.PodSpec) {
	podSpec.InitContainers = append(podSpec.InitContainers, corev1.Container{
		Name:    "precreate-subpaths",
		Image:   "registry.access.redhat.com/ubi8/ubi-minimal",
		Command: []string{"/usr/bin/mkdir"},
		Args: []string{
			"-p",
			"-v",
			"-m",
			"777",
			"/tmp/che-workspaces/" + names.WorkspaceId,
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

func mergeWorkspaceAdditions(workspaceDeployment *appsv1.Deployment, componentInstanceStatuses []ComponentInstanceStatus, k8sObjects []runtime.Object) error {
	workspacePodAdditions := []corev1.PodTemplateSpec{}
	for _, componentInstanceStatus := range componentInstanceStatuses {
		if componentInstanceStatus.WorkspacePodAdditions == nil {
			continue
		}
		workspacePodAdditions = append(workspacePodAdditions, *componentInstanceStatus.WorkspacePodAdditions)
	}
	workspacePodTemplate := &workspaceDeployment.Spec.Template
	containers := map[string]corev1.Container{}
	initContainers := map[string]corev1.Container{}
	volumes := map[string]corev1.Volume{}
	pullSecrets := map[string]corev1.LocalObjectReference{}

	for _, addition := range workspacePodAdditions {
		for annotKey, annotValue := range addition.Annotations {
			workspacePodTemplate.Annotations[annotKey] = annotValue
		}

		for labelKey, labelValue := range addition.Labels {
			workspacePodTemplate.Labels[labelKey] = labelValue
		}

		for _, container := range addition.Spec.Containers {
			if _, exists := containers[container.Name]; exists {
				return errors.New("Duplicate conainers in the workspace definition: " + container.Name)
			}
			containers[container.Name] = container
			workspacePodTemplate.Spec.Containers = append(workspacePodTemplate.Spec.Containers, container)
		}

		for _, container := range addition.Spec.InitContainers {
			if _, exists := initContainers[container.Name]; exists {
				return errors.New("Duplicate init conainers in the workspace definition: " + container.Name)
			}
			initContainers[container.Name] = container
			workspacePodTemplate.Spec.InitContainers = append(workspacePodTemplate.Spec.InitContainers, container)
		}

		for _, volume := range addition.Spec.Volumes {
			if _, exists := volumes[volume.Name]; exists {
				return errors.New("Duplicate volumes in the workspace definition: " + volume.Name)
			}
			volumes[volume.Name] = volume
			workspacePodTemplate.Spec.Volumes = append(workspacePodTemplate.Spec.Volumes, volume)
		}

		for _, pullSecret := range addition.Spec.ImagePullSecrets {
			if _, exists := pullSecrets[pullSecret.Name]; exists {
				continue
			}
			pullSecrets[pullSecret.Name] = pullSecret
			workspacePodTemplate.Spec.ImagePullSecrets = append(workspacePodTemplate.Spec.ImagePullSecrets, pullSecret)
		}
	}
	workspacePodTemplate.Labels[DEPLOYMENT_NAME_LABEL] = workspaceDeployment.Name
	for _, externalObject := range k8sObjects {
		service, isAService := externalObject.(*corev1.Service)
		if isAService {
			service.Spec.Selector[DEPLOYMENT_NAME_LABEL] = workspaceDeployment.Name
		}
	}
	return nil
}
