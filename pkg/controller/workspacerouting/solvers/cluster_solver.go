package solvers

import (
	"fmt"

	"github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	"github.com/che-incubator/che-workspace-operator/pkg/config"
	corev1 "k8s.io/api/core/v1"
)

type ClusterSolver struct{}

var _ RoutingSolver = (*ClusterSolver)(nil)

func (s *ClusterSolver) GetSpecObjects(spec v1alpha1.WorkspaceRoutingSpec, workspaceMeta WorkspaceMetadata) RoutingObjects {
	services := getServicesForEndpoints(spec.Endpoints, workspaceMeta)

	return RoutingObjects{
		Services: services,
	}
}

func (s *ClusterSolver) GetExposedEndpoints(
	endpoints map[string]v1alpha1.EndpointList,
	routingObj RoutingObjects) (exposedEndpoints map[string]v1alpha1.ExposedEndpointList, ready bool, err error) {

	exposedEndpoints = map[string]v1alpha1.ExposedEndpointList{}

	for machineName, machineEndpoints := range endpoints {
		for _, endpoint := range machineEndpoints {
			if endpoint.Attributes[v1alpha1.PUBLIC_ENDPOINT_ATTRIBUTE] != "true" {
				continue
			}
			url, err := resolveServiceHostnameForEndpoint(endpoint, routingObj.Services)
			if err != nil {
				return nil, false, err
			}
			exposedEndpoints[machineName] = append(exposedEndpoints[machineName], v1alpha1.ExposedEndpoint{
				Name:       endpoint.Name,
				Url:        url,
				Attributes: endpoint.Attributes,
			})
		}
	}

	return exposedEndpoints, true, nil
}

func resolveServiceHostnameForEndpoint(endpoint v1alpha1.Endpoint, services []corev1.Service) (string, error) {
	for _, service := range services {
		if service.Annotations[config.WorkspaceDiscoverableServiceAnnotation] == "true" {
			continue
		}
		for _, servicePort := range service.Spec.Ports {
			if int64(servicePort.Port) == endpoint.Port {
				return getHostnameFromService(service, servicePort.Port), nil
			}
		}
	}
	return "", fmt.Errorf("could not find service for endpoint %s", endpoint.Name)
}

func getHostnameFromService(service corev1.Service, port int32) string {
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", service.Name, service.Namespace, port)
}
