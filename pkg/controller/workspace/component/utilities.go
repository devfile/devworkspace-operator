package component

import (
	"strings"

	workspaceApi "github.com/che-incubator/che-workspace-crd-operator/pkg/apis/workspace/v1alpha1"
	. "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/config"
	. "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/model"
)

func emptyIfNil(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

func machineServiceName(wkspProps WorkspaceProperties, machineName string) string {
	return "server"+strings.ReplaceAll(wkspProps.WorkspaceId, "workspace", "") + "-" + machineName
}

func endpointPortsToInts(endpoints []workspaceApi.Endpoint) []int {
	ports := []int {}
	for _, endpint := range endpoints {
		ports = append(ports, int(endpint.Port))
	}
	return ports
}

func ingressHostName(name string, wkspProperties WorkspaceProperties) string {
	return name + "-" + wkspProperties.Namespace + "." + ControllerCfg.GetIngressGlobalDomain()
}
