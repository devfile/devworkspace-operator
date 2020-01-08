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
	"strings"
	"github.com/eclipse/che-plugin-broker/model"
	"regexp"
	"strconv"

	workspaceApi "github.com/che-incubator/che-workspace-crd-operator/pkg/apis/workspace/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	k8sModelUtils "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/modelutils/k8s"

	. "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/model"
	. "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/config"
)

func setupDockerImageComponent(names WorkspaceProperties, commands []workspaceApi.CommandSpec, component *workspaceApi.ComponentSpec) (*ComponentInstanceStatus, error) {
	componentInstanceStatus := &ComponentInstanceStatus{
		Machines: map[string]MachineDescription{},
		Endpoints: []workspaceApi.Endpoint {},
		ContributedRuntimeCommands: []CheWorkspaceCommand {},
	}

	podTemplate := &corev1.PodTemplateSpec{}
	componentInstanceStatus.WorkspacePodAdditions = podTemplate
	componentInstanceStatus.ExternalObjects = []runtime.Object{}

	var machineName string
	if component.Alias == nil {
		re := regexp.MustCompile(`[^-a-zA-Z0-9_]`)
		machineName = re.ReplaceAllString(*component.Image, "-")
	} else {
		machineName = *component.Alias
	}

	var exposedPorts []int = endpointPortsToInts(component.Endpoints)

	var limitOrDefault string

	if *component.MemoryLimit == "" {
		limitOrDefault = "128M"
	} else {
		limitOrDefault = *component.MemoryLimit
	}

	limit, err := resource.ParseQuantity(limitOrDefault)
	if err != nil {
		return nil, err
	}

	volumeMounts := createVolumeMounts(names, component.MountSources, component.Volumes, []model.Volume{})

	var envVars []corev1.EnvVar
	for _, envVarDef := range component.Env {
		envVars = append(envVars, corev1.EnvVar{
			Name:  envVarDef.Name,
			Value: strings.ReplaceAll(envVarDef.Value, "$(CHE_PROJECTS_ROOT)", "/projects"),
		})
	}
	envVars = append(envVars, corev1.EnvVar{
		Name:  "CHE_MACHINE_NAME",
		Value: machineName,
	})
	container := corev1.Container{
		Name:            machineName,
		Image:           *component.Image,
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
		VolumeMounts: volumeMounts,
		Env:          append(envVars, commonEnvironmentVariables(names)...),
	}
	if component.Command != nil {
		container.Command = *component.Command
	}
	if component.Args != nil {
		container.Args = *component.Args
	}

// TODO selector, etc ....

	podTemplate.Spec.Containers = append(podTemplate.Spec.Containers, container)

	for _, service := range createK8sServicesForMachines(names, machineName, exposedPorts) {
		componentInstanceStatus.ExternalObjects = append(componentInstanceStatus.ExternalObjects, &service)
	}

	componentInstanceStatus.Endpoints = component.Endpoints

	machineAttributes := map[string]string {}
	if limitAsInt64, canBeConverted := limit.AsInt64(); canBeConverted {
		machineAttributes[MEMORY_LIMIT_ATTRIBUTE] = strconv.FormatInt(limitAsInt64, 10)
		machineAttributes[MEMORY_REQUEST_ATTRIBUTE] = strconv.FormatInt(limitAsInt64, 10)
	}
	machineAttributes[CONTAINER_SOURCE_ATTRIBUTE] = RECIPE_CONTAINER_SOURCE
	componentInstanceStatus.Machines[machineName] = MachineDescription {
		MachineAttributes: machineAttributes,
    Ports: exposedPorts,
	}
	
	for _, command := range commands {
		if len(command.Actions) == 0 {
			continue
		}
		action := command.Actions[0]
		if component.Alias == nil ||
			action.Component == nil ||
		  *action.Component != *component.Alias {
			continue
		}
		attributes := map[string]string{
			COMMAND_WORKING_DIRECTORY_ATTRIBUTE: interpolate(emptyIfNil(action.Workdir), names),
			COMMAND_ACTION_REFERENCE_ATTRIBUTE:  emptyIfNil(action.Reference),
			COMMAND_ACTION_REFERENCE_CONTENT_ATTRIBUTE:  emptyIfNil(action.ReferenceContent),
			COMMAND_MACHINE_NAME_ATTRIBUTE:      machineName,
			COMPONENT_ALIAS_COMMAND_ATTRIBUTE:   *action.Component,
		}
		for attrName, attrValue := range command.Attributes {
			attributes[attrName] = attrValue
		}
		componentInstanceStatus.ContributedRuntimeCommands = append(componentInstanceStatus.ContributedRuntimeCommands,
			CheWorkspaceCommand{
				Name:        command.Name,
				CommandLine: emptyIfNil(action.Command),
				Type:        action.Type,
				Attributes: attributes,
			})
	}

	return componentInstanceStatus, nil
}
