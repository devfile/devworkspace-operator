package workspace

import (
	"strconv"
	"strings"

	workspaceApi "github.com/che-incubator/che-workspace-crd-operator/pkg/apis/workspace/v1alpha1"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func join(sep string, parts ...string) string {
	return strings.Join(parts, sep)
}

func BoolStringPtr(val bool) *string { 
	var str string
	if val {
		str = "true"
	} else {
		str = "false"
	}
	return &str
}

func emptyIfNil(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

func portAsString(port int) string {
	return strconv.FormatInt(int64(port), 10)
}

func machineServiceName(wkspProps workspaceProperties, machineName string) string {
	return join("-",
		"server"+strings.ReplaceAll(wkspProps.workspaceId, "workspace", ""),
		machineName)
}

func servicePortAndProtocol(port int) string {
	return join("/", portAsString(port), strings.ToLower(string(servicePortProtocol)))
}

func EndpointPortsToInts(endpoints []workspaceApi.Endpoint) []int {
	ports := []int {}
	for _, endpint := range endpoints {
		ports = append(ports, int(endpint.Port))
	}
	return ports
}

func ingressHostName(name string, wkspProperties workspaceProperties) string {
	return name + "-" + wkspProperties.namespace + "." + controllerConfig.getIngressGlobalDomain()
}

func IsOpenShift() (bool, error) {
	kubeconfig, err := config.GetConfig()
	if err != nil {
		return false, err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(kubeconfig)
	if err != nil {
		return false, err
	}
	apiList, err := discoveryClient.ServerGroups()
	if err != nil {
		return false, err
	}
	apiGroups := apiList.Groups
	for i := 0; i < len(apiGroups); i++ {
		if apiGroups[i].Name == "route.openshift.io" {
			return true, nil
		}
	}
	return false, nil
}
