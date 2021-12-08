package conversion

import (
	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
)

func convertDevfileEndpoint(dwEndpoint dw.Endpoint) v1alpha1.Endpoint {
	return v1alpha1.Endpoint{
		Name:       dwEndpoint.Name,
		TargetPort: dwEndpoint.TargetPort,
		Exposure:   v1alpha1.EndpointExposure(dwEndpoint.Exposure),
		Protocol:   v1alpha1.EndpointProtocol(dwEndpoint.Protocol),
		Secure:     dwEndpoint.Secure,
		Path:       dwEndpoint.Path,
		Attributes: v1alpha1.Attributes(dwEndpoint.Attributes),
	}
}

func ConvertAllDevfileEndpoints(dwEndpoint []dw.Endpoint) v1alpha1.EndpointList {
	var convertedEndpoints v1alpha1.EndpointList

	for _, endpoint := range dwEndpoint {
		convertedEndpoints = append(convertedEndpoints, convertDevfileEndpoint(endpoint))
	}

	return convertedEndpoints
}
