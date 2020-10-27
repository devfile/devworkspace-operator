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

package adaptor

import (
	"encoding/json"
	"fmt"

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha1"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/eclipse/che-plugin-broker/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getArtifactsBrokerComponent(workspaceId, namespace string, components []devworkspace.Component) (*v1alpha1.ComponentDescription, *corev1.ConfigMap, error) {
	const (
		configMapVolumeName = "broker-config-volume"
		configMapMountPath  = "/broker-config"
		configMapDataName   = "config.json"
	)
	configMapName := common.PluginBrokerConfigmapName(workspaceId)
	brokerImage := config.ControllerCfg.GetPluginArtifactsBrokerImage()
	brokerContainerName := "plugin-artifacts-broker"

	var fqns []model.PluginFQN
	for _, component := range components {
		fqns = append(fqns, getPluginFQN(*component.Plugin))
	}
	cmData, err := json.Marshal(fqns)
	if err != nil {
		return nil, nil, err
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
			Labels: map[string]string{
				config.WorkspaceIDLabel: workspaceId,
			},
		},
		Data: map[string]string{
			configMapDataName: string(cmData),
		},
	}

	cmMode := int32(0644)
	// Define volumes used by plugin broker
	cmVolume := corev1.Volume{
		Name: configMapVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMapName,
				},
				DefaultMode: &cmMode,
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
			MountPath: config.PluginsMountPath,
			Name:      config.ControllerCfg.GetWorkspacePVCName(),
			SubPath:   workspaceId + "/plugins",
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
			fmt.Sprintf("%s:%s:%s", workspaceId, "default", "anonymous"),
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

	brokerComponent := &v1alpha1.ComponentDescription{
		Name: "artifacts-broker",
		PodAdditions: v1alpha1.PodAdditions{
			InitContainers: []corev1.Container{initContainer},
			Volumes:        []corev1.Volume{cmVolume},
		},
	}

	return brokerComponent, cm, nil
}

func isArtifactsBrokerNecessary(metas []model.PluginMeta) bool {
	for _, meta := range metas {
		if len(meta.Spec.Extensions) > 0 {
			return true
		}
	}
	return false
}
