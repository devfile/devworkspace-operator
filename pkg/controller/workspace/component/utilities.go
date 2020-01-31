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
	k8sModelUtils "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/modelutils/k8s"
	"github.com/eclipse/che-plugin-broker/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"

	workspaceApi "github.com/che-incubator/che-workspace-crd-operator/pkg/apis/workspace/v1alpha1"
	. "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/model"
)

func emptyIfNil(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

func machineServiceName(wkspProps WorkspaceProperties, machineName string) string {
	return "server" + strings.ReplaceAll(wkspProps.WorkspaceId, "workspace", "") + "-" + machineName
}

func endpointPortsToInts(endpoints []workspaceApi.Endpoint) []int {
	ports := []int{}
	for _, endpint := range endpoints {
		ports = append(ports, int(endpint.Port))
	}
	return ports
}

func createVolumeMounts(workspaceProps WorkspaceProperties, mountSources *bool, devfileVolumes []workspaceApi.Volume, pluginVolumes []model.Volume) []corev1.VolumeMount {
	var volumeMounts []corev1.VolumeMount
	volumeName := "claim-che-workspace"

	for _, volDef := range devfileVolumes {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			MountPath: volDef.ContainerPath,
			Name:      volumeName,
			SubPath:   workspaceProps.WorkspaceId + "/" + volDef.Name + "/",
		})
	}
	for _, volDef := range pluginVolumes {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			MountPath: volDef.MountPath,
			Name:      volumeName,
			SubPath:   workspaceProps.WorkspaceId + "/" + volDef.Name + "/",
		})
	}

	if mountSources != nil && *mountSources {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			MountPath: DefaultProjectsSourcesRoot,
			Name:      volumeName,
			SubPath:   workspaceProps.WorkspaceId + DefaultProjectsSourcesRoot,
		})
	}

	return volumeMounts
}

func createK8sServicesForMachines(wkspProps WorkspaceProperties, machineName string, exposedPorts []int) []corev1.Service {
	services := []corev1.Service{}
	servicePorts := k8sModelUtils.BuildServicePorts(exposedPorts, corev1.ProtocolTCP)
	serviceName := machineServiceName(wkspProps, machineName)
	if len(servicePorts) > 0 {
		service := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: wkspProps.Namespace,
				Annotations: map[string]string{
					"org.eclipse.che.machine.name": machineName,
				},
				Labels: map[string]string{
					WorkspaceIDLabel: wkspProps.WorkspaceId,
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					CheOriginalNameLabel: CheOriginalName,
					WorkspaceIDLabel:     wkspProps.WorkspaceId,
				},
				Type:  corev1.ServiceTypeClusterIP,
				Ports: servicePorts,
			},
		}
		services = append(services, service)
	}
	return services
}

func interpolate(someString string, wkspProps WorkspaceProperties) string {
	for _, envVar := range commonEnvironmentVariables(wkspProps) {
		someString = strings.ReplaceAll(someString, "${"+envVar.Name+"}", envVar.Value)
	}
	return someString
}
