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
	"encoding/json"

	workspaceApi "github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	"github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/config"
	workspace "github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/utils"
	metadataBroker "github.com/eclipse/che-plugin-broker/brokers/metadata"
	brokerModel "github.com/eclipse/che-plugin-broker/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"fmt"

	"github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/model"
)

func setupChePlugins(wkspCtx model.WorkspaceContext, components []workspaceApi.ComponentSpec) ([]model.ComponentInstanceStatus, error) {
	var componentInstanceStatuses []model.ComponentInstanceStatus

	broker := metadataBroker.NewBroker(true)
	metas, err := getMetasForComponents(components)
	if err != nil {
		return nil, err
	}

	plugins, err := broker.ProcessPlugins(metas)
	if err != nil {
		return nil, err
	}

	for _, plugin := range plugins {
		component, err := convertToComponentInstanceStatus(plugin, wkspCtx)
		if err != nil {
			return nil, err
		}
		componentInstanceStatuses = append(componentInstanceStatuses, *component)
	}

	if isArtifactsBrokerNecessary(metas) {
		artifactsBroker, err := getArtifactsBrokerObjects(wkspCtx, components)
		if err != nil {
			return nil, err
		}
		componentInstanceStatuses = append(componentInstanceStatuses, artifactsBroker)
	}

	return componentInstanceStatuses, nil
}

func getArtifactsBrokerObjects(wkspCtx model.WorkspaceContext, components []workspaceApi.ComponentSpec) (model.ComponentInstanceStatus, error) {
	var brokerComponent model.ComponentInstanceStatus

	const (
		configMapVolumeName = "broker-config-volume"
		configMapMountPath  = "/broker-config"
		configMapDataName   = "config.json"
	)
	configMapName := fmt.Sprintf("%s.broker-config-map", wkspCtx.WorkspaceId)
	brokerImage := config.ControllerCfg.GetPluginArtifactsBrokerImage()
	brokerContainerName := workspace.GetContainerNameFromImage(brokerImage)

	// Define plugin broker configmap
	var fqns []brokerModel.PluginFQN
	for _, component := range components {
		fqns = append(fqns, getPluginFQN(component))
	}
	cmData, err := json.Marshal(fqns)
	if err != nil {
		return brokerComponent, err
	}
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: wkspCtx.Namespace,
			Labels: map[string]string{
				model.WorkspaceIDLabel: wkspCtx.WorkspaceId,
			},
		},
		Data: map[string]string{
			configMapDataName: string(cmData),
		},
	}

	// Define volumes used by plugin broker
	cmVolume := corev1.Volume{
		Name: configMapVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMapName,
				},
			},
		},
	}

	cmVolumeMounts := []corev1.VolumeMount{
		{
			MountPath: configMapMountPath,
			Name:      configMapVolumeName,
			ReadOnly:  true,
		},
		{
			MountPath: "/plugins",
			Name:      config.ControllerCfg.GetWorkspacePVCName(),
			SubPath:   wkspCtx.WorkspaceId + "/plugins",
		},
	}

	initContainer := corev1.Container{
		Name:                     brokerContainerName,
		Image:                    brokerImage,
		ImagePullPolicy:          corev1.PullPolicy(config.ControllerCfg.GetSidecarPullPolicy()),
		VolumeMounts:             cmVolumeMounts,
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
		Args: []string{
			"--disable-push",
			"--runtime-id",
			fmt.Sprintf("%s:%s:%s", wkspCtx.WorkspaceId, "default", "anonymous"),
			"--registry-address",
			config.ControllerCfg.GetPluginRegistry(),
			"--metas",
			fmt.Sprintf("%s/%s", configMapMountPath, configMapDataName),
		},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("150Mi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("150Mi"),
			},
		},
	}

	brokerComponent = model.ComponentInstanceStatus{
		WorkspacePodAdditions: &corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				InitContainers: []corev1.Container{initContainer},
				Volumes:        []corev1.Volume{cmVolume},
			},
		},
		ExternalObjects: []runtime.Object{&cm},
	}

	return brokerComponent, err
}

func isArtifactsBrokerNecessary(metas []brokerModel.PluginMeta) bool {
	for _, meta := range metas {
		if len(meta.Spec.Extensions) > 0 {
			return true
		}
	}
	return false
}
