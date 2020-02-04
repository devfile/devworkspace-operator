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
	"errors"
	"github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/server"
	"strconv"
	"strings"

	"github.com/eclipse/che-plugin-broker/utils"

	workspaceApi "github.com/che-incubator/che-workspace-crd-operator/pkg/apis/workspace/v1alpha1"
	k8sModelUtils "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/modelutils/k8s"
	metadataBroker "github.com/eclipse/che-plugin-broker/brokers/metadata"
	commonBroker "github.com/eclipse/che-plugin-broker/common"
	"github.com/eclipse/che-plugin-broker/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"fmt"
	. "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/config"
	. "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/model"
)

// TODO : change this because we don't expect plugin metas anymore, but plugin FQNs in the config maps
func setupPluginInitContainers(names WorkspaceProperties, podSpec *corev1.PodSpec, pluginFQNs []model.PluginFQN) ([]runtime.Object, error) {
	var k8sObjects []runtime.Object

	type initContainerDef struct {
		imageName  string
		pluginFQNs []model.PluginFQN
	}

	for _, def := range []initContainerDef{
		{
			imageName:  "che.workspace.plugin_broker.init.image",
			pluginFQNs: []model.PluginFQN{},
		},
		{
			imageName:  "che.workspace.plugin_broker.unified.image",
			pluginFQNs: pluginFQNs,
		},
	} {
		brokerImage := ControllerCfg.GetProperty(def.imageName)
		if brokerImage == nil {
			return nil, errors.New("Unknown broker docker image for : " + def.imageName)
		}

		volumeMounts := []corev1.VolumeMount{
			corev1.VolumeMount{
				MountPath: "/plugins/",
				Name:      ControllerCfg.GetWorkspacePVCName(),
				SubPath:   names.WorkspaceId + "/plugins/",
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
			fmt.Sprintf("%s:%s:%s", names.WorkspaceId, "default", "anonymous"),
			"--registry-address",
			ControllerCfg.GetPluginRegistry(),
		}

		if len(def.pluginFQNs) > 0 {

			// TODO: See how the unified broker is defined in the yaml
			// and define it the same way here.
			// See also how it is that we do not put volume =>
			// => Log on the operator side to see what there is in PluginFQNs

			configMapName := containerName + "-broker-config-map"
			configMapVolume := containerName + "-broker-config-volume"
			configMapContent, err := json.MarshalIndent(pluginFQNs, "", "")
			if err != nil {
				return nil, err
			}

			configMap := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: names.Namespace,
					Labels: map[string]string{
						WorkspaceIDLabel: names.WorkspaceId,
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

			ImagePullPolicy:          corev1.PullAlways,
			VolumeMounts:             volumeMounts,
			TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
		})
	}
	return k8sObjects, nil
}

func setupChePlugin(names WorkspaceProperties, component *workspaceApi.ComponentSpec) (*ComponentInstanceStatus, error) {
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

	pluginMeta, err := utils.GetPluginMeta(pluginFQN, ControllerCfg.GetPluginRegistry(), theIoUtil)
	if err != nil {
		return nil, err
	}

	pluginMetas := []model.PluginMeta{*pluginMeta}
	err = utils.ResolveRelativeExtensionPaths(pluginMetas, ControllerCfg.GetPluginRegistry())
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
		metadataBroker.AddPluginRunnerRequirements(*pluginMeta, theRand, true)
	}

	componentInstanceStatus := &ComponentInstanceStatus{
		Containers:                 map[string]ContainerDescription{},
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
		containerName := containerDef.Name

		var exposedPorts []int = exposedPortsToInts(containerDef.Ports)

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
			Value: containerName,
		})
		container := corev1.Container{
			Name:            containerName,
			Image:           containerDef.Image,
			ImagePullPolicy: corev1.PullPolicy(ControllerCfg.GetSidecarPullPolicy()),
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

		for _, service := range createK8sServicesForContainers(names, containerName, exposedPorts) {
			componentInstanceStatus.ExternalObjects = append(componentInstanceStatus.ExternalObjects, &service)
		}

		for _, endpointDef := range chePlugin.Endpoints {
			attributes := map[workspaceApi.EndpointAttribute]string{}
			if endpointDef.Public {
				attributes[workspaceApi.PUBLIC_ENDPOINT_ATTRIBUTE] = "true"
			} else {
				attributes[workspaceApi.PUBLIC_ENDPOINT_ATTRIBUTE] = "false"
			}
			for name, value := range endpointDef.Attributes {
				attributes[workspaceApi.EndpointAttribute(name)] = value
			}
			if attributes[workspaceApi.PROTOCOL_ENDPOINT_ATTRIBUTE] == "" {
				attributes[workspaceApi.PROTOCOL_ENDPOINT_ATTRIBUTE] = "http"
			}
			endpoint := workspaceApi.Endpoint{
				Name:       endpointDef.Name,
				Port:       int64(endpointDef.TargetPort),
				Attributes: attributes,
			}
			componentInstanceStatus.Endpoints = append(componentInstanceStatus.Endpoints, endpoint)
		}

		containerAttributes := map[string]string{}
		if limitAsInt64, canBeConverted := limit.AsInt64(); canBeConverted {
			containerAttributes[server.MEMORY_LIMIT_ATTRIBUTE] = strconv.FormatInt(limitAsInt64, 10)
			containerAttributes[server.MEMORY_REQUEST_ATTRIBUTE] = strconv.FormatInt(limitAsInt64, 10)
		}
		containerAttributes[server.CONTAINER_SOURCE_ATTRIBUTE] = server.TOOL_CONTAINER_SOURCE
		containerAttributes[server.PLUGIN_MACHINE_ATTRIBUTE] = chePlugin.ID

		componentInstanceStatus.Containers[containerName] = ContainerDescription{
			Attributes: containerAttributes,
			Ports:      exposedPorts,
		}

		for _, command := range containerDef.Commands {
			componentInstanceStatus.ContributedRuntimeCommands = append(componentInstanceStatus.ContributedRuntimeCommands,
				CheWorkspaceCommand{
					Name:        command.Name,
					CommandLine: strings.Join(command.Command, " "),
					Type:        "custom",
					Attributes: map[string]string{
						server.COMMAND_WORKING_DIRECTORY_ATTRIBUTE: interpolate(command.WorkingDir, names),
						server.COMMAND_MACHINE_NAME_ATTRIBUTE:      containerName,
					},
				})
		}
	}

	return componentInstanceStatus, nil
}

func exposedPortsToInts(exposedPorts []model.ExposedPort) []int {
	ports := []int{}
	for _, exposedPort := range exposedPorts {
		ports = append(ports, exposedPort.ExposedPort)
	}
	return ports
}
