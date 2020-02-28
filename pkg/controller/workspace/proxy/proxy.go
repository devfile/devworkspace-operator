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

package proxy

import (
	"fmt"
	"github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	"github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/config"
	"github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/model"
	"github.com/che-incubator/che-workspace-operator/pkg/specutils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"strconv"
)

const proxyServiceAcctAnnotationKeyFmt string = "serviceaccounts.openshift.io/oauth-redirectreference.%s-%s"
const proxyServiceAcctAnnotationValueFmt string = `{"kind":"OAuthRedirectReference","apiVersion":"v1","reference":{"kind":"Route","name":"%s"}}`

type proxyEndpoint struct {
	upstreamEndpoint       v1alpha1.Endpoint
	publicEndpoint         v1alpha1.Endpoint
	publicEndpointHttpPort int64
}

func AddProxyToDeployment(
	wkspCtx model.WorkspaceContext,
	deployment *appsv1.Deployment,
	k8sObjects *[]runtime.Object,
	componentInstanceStatuses *[]model.ComponentInstanceStatus) error {

	serviceAcct, err := findProxyServiceAccount(*k8sObjects)
	if err != nil {
		return err
	}

	var endpointsToProxy []v1alpha1.Endpoint
	// Get endpoints we need to recreate as proxied endpoints, and remove those from componentInstanceStatuses
	for idx, component := range *componentInstanceStatuses {
		toProxy, noProxy := getProxyEndpoints(component)
		(*componentInstanceStatuses)[idx].Endpoints = noProxy
		endpointsToProxy = append(endpointsToProxy, toProxy...)
	}

	*k8sObjects = removeProxiedServicePoints(*k8sObjects, endpointsToProxy)
	// TODO: Do componentStatuses[].Containers[].Ports also need to be updated?

	var proxyContainers []corev1.Container
	proxyEndpoints := getProxyPortMappings(wkspCtx, endpointsToProxy)
	for _, proxy := range proxyEndpoints {
		proxyContainers = append(proxyContainers, createProxyContainer(proxy, serviceAcct.Name))
	}

	proxyVolumes := getProxyVolumes(proxyContainers)
	proxyService := getServiceForContainerPorts(wkspCtx.WorkspaceId, wkspCtx.Namespace, proxyContainers, deployment.Labels)

	annotateServiceAccount(wkspCtx, serviceAcct, proxyService)

	deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, proxyContainers...)
	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, proxyVolumes...)
	*k8sObjects = append(*k8sObjects, &proxyService)

	proxyComponent := createProxyComponentStatus(proxyContainers, proxyEndpoints)
	*componentInstanceStatuses = append(*componentInstanceStatuses, proxyComponent)

	return nil
}

func createProxyComponentStatus(containers []corev1.Container, proxyEndpoints []proxyEndpoint) model.ComponentInstanceStatus {
	containerMetas := map[string]model.ContainerDescription{}
	for _, container := range containers {
		var ports []int
		for _, port := range container.Ports {
			ports = append(ports, int(port.ContainerPort))
		}
		containerMetas[container.Name] = model.ContainerDescription{
			Attributes: map[string]string{
				"source": "tool",
			},
			Ports:      ports,
		}
	}

	var endpoints []v1alpha1.Endpoint
	for _, proxyEndpoint := range proxyEndpoints {
		endpoints = append(endpoints, proxyEndpoint.publicEndpoint)
	}

	return model.ComponentInstanceStatus{
		Containers:                 containerMetas,
		WorkspacePodAdditions:      nil,
		ExternalObjects:            nil,
		Endpoints:                  endpoints,
		ContributedRuntimeCommands: nil,
	}
}

func annotateServiceAccount(wkspCtx model.WorkspaceContext, serviceAcct *corev1.ServiceAccount, service corev1.Service) {
	if serviceAcct.Annotations == nil {
		serviceAcct.Annotations = make(map[string]string)
	}
	for _, port := range service.Spec.Ports {
		portNum := int64(port.Port)
		// TODO: Figure out naming once and for all
		ingressName := specutils.IngressName(service.Name, portNum)
		annotKey := fmt.Sprintf(proxyServiceAcctAnnotationKeyFmt, wkspCtx.WorkspaceId, strconv.FormatInt(portNum, 10))
		annotVal := fmt.Sprintf(proxyServiceAcctAnnotationValueFmt, ingressName)
		serviceAcct.Annotations[annotKey] = annotVal
	}
}

func getProxyPortMappings(wkspCtx model.WorkspaceContext, endpointsToProxy []v1alpha1.Endpoint) []proxyEndpoint {
	proxyHttpsPort := 4400
	proxyHttpPort := int64(4180)
	var proxyEndpoints []proxyEndpoint
	for _, toProxy := range endpointsToProxy {
		proxyEndpoint := proxyEndpoint{
			upstreamEndpoint: toProxy,
			publicEndpoint: v1alpha1.Endpoint{
				Attributes: map[v1alpha1.EndpointAttribute]string{
					v1alpha1.PUBLIC_ENDPOINT_ATTRIBUTE: "true",
					v1alpha1.SECURE_ENDPOINT_ATTRIBUTE: "true",
					v1alpha1.TYPE_ENDPOINT_ATTRIBUTE:   "ide",
				},
				Name: specutils.ProxyRouteName(wkspCtx.WorkspaceId, toProxy),
				Port: int64(proxyHttpsPort),
			},
			publicEndpointHttpPort: proxyHttpPort,
		}
		proxyEndpoints = append(proxyEndpoints, proxyEndpoint)
		proxyHttpsPort++
		proxyHttpPort++
	}
	return proxyEndpoints
}

func createProxyContainer(endpoint proxyEndpoint, serviceAcctName string) corev1.Container {
	return corev1.Container{
		Name: specutils.ProxyContainerName(endpoint.upstreamEndpoint),
		Ports: []corev1.ContainerPort{
			{
				//Name:          endpoint.upstreamEndpoint.Name,
				ContainerPort: int32(endpoint.publicEndpoint.Port),
				Protocol:      corev1.ProtocolTCP,
			},
		},
		ImagePullPolicy: corev1.PullPolicy(config.ControllerCfg.GetSidecarPullPolicy()),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "proxy-tls",
				MountPath: "/etc/tls/private",
			},
		},
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
		Image:                    "openshift/oauth-proxy:latest",
		Args: []string{
			"--https-address=:" + strconv.FormatInt(endpoint.publicEndpoint.Port, 10),
			"--http-address=127.0.0.1:" + strconv.FormatInt(endpoint.publicEndpointHttpPort, 10),
			"--provider=openshift",
			"--openshift-service-account=" + serviceAcctName,
			"--upstream=http://localhost:" + strconv.FormatInt(endpoint.upstreamEndpoint.Port, 10),
			"--tls-cert=/etc/tls/private/tls.crt",
			"--tls-key=/etc/tls/private/tls.key",
			"--cookie-secret=SECRET",
		},
	}
}

func getProxyVolumes(containers []corev1.Container) []corev1.Volume {
	var volumes []corev1.Volume
	volumeNames := map[string]bool{}
	var volumeDefaultMode int32 = 420
	for _, container := range containers {
		for _, volumeMount := range container.VolumeMounts {
			if volumeNames[volumeMount.Name] {
				continue
			}
			volume := corev1.Volume{
				Name: volumeMount.Name,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  volumeMount.Name,
						DefaultMode: &volumeDefaultMode,
					},
				},
			}
			volumes = append(volumes, volume)
			volumeNames[volumeMount.Name] = true
		}
	}
	return volumes
}

func findProxyServiceAccount(k8sObjects []runtime.Object) (*corev1.ServiceAccount, error) {
	for _, obj := range k8sObjects {
		if serviceAcct, isServiceAcct := obj.(*corev1.ServiceAccount); isServiceAcct {
			return serviceAcct, nil
		}
	}
	return nil, fmt.Errorf("no service account associated with workspace")
}

func getProxyEndpoints(component model.ComponentInstanceStatus) (proxied, unmodified []v1alpha1.Endpoint) {
	for _, endpoint := range component.Endpoints {
		if specutils.EndpointNeedsProxy(endpoint) {
			proxied = append(proxied, endpoint)
		} else {
			unmodified = append(unmodified, endpoint)
		}
	}
	return
}

func removeProxiedServicePoints(k8sObjects []runtime.Object, endpointsToRemove []v1alpha1.Endpoint) []runtime.Object {
	var updatedObjects []runtime.Object
	for _, obj := range k8sObjects {
		service, isService := obj.(*corev1.Service)
		if !isService {
			updatedObjects = append(updatedObjects, obj)
			continue
		}

		var unproxiedServicePorts []corev1.ServicePort
		for _, port := range service.Spec.Ports {
			if !isServicePortProxied(port, endpointsToRemove) {
				unproxiedServicePorts = append(unproxiedServicePorts, port)
			}
		}
		if len(unproxiedServicePorts) > 0 {
			// Some endpoints for this service are unproxied
			service.Spec.Ports = unproxiedServicePorts
			updatedObjects = append(updatedObjects, service)
		}
	}
	return updatedObjects
}

func isServicePortProxied(servicePort corev1.ServicePort, proxyEndpoints []v1alpha1.Endpoint) bool {
	for _, proxyEndpoint := range proxyEndpoints {
		if proxyEndpoint.Port == int64(servicePort.Port) {
			return true
		}
	}
	return false
}

func getServiceForContainerPorts(name, namespace string, containers []corev1.Container, deploymentLabels map[string]string) corev1.Service {
	var servicePorts []corev1.ServicePort
	for _, container := range containers {
		for _, port := range container.Ports {
			servicePorts = append(servicePorts, corev1.ServicePort{
				Name:     specutils.ServicePortName(int(port.ContainerPort)),
				Port:     port.ContainerPort,
				Protocol: corev1.ProtocolTCP,
			})
		}
	}
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			// TODO: Ugly workaround because naming things is complicated
			Name:      specutils.ContainerServiceName(name+"-oauth-proxy", "theia"),
			Namespace: namespace,
			Annotations: map[string]string{
				"service.alpha.openshift.io/serving-cert-secret-name": "proxy-tls",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: deploymentLabels,
			Type:     corev1.ServiceTypeClusterIP,
			Ports:    servicePorts,
		},
	}

	return service
}
