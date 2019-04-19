package workspace

import (
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"strconv"
	"strings"
)

func join(sep string, parts ...string) string {
	return strings.Join(parts, sep)
}

func portAsString(port int) string {
	return strconv.FormatInt(int64(port), 10)
}

func servicePortName(port int) string {
	return "srv-" + portAsString(port)
}

func servicePortAndProtocol(port int) string {
	return join("/", portAsString(port), strings.ToLower(string(servicePortProtocol)))
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
	log.Info("In IsOpenshift", "apiGroups", apiGroups)
	for i := 0; i < len(apiGroups); i++ {
			if apiGroups[i].Name == "route.openshift.io" {
				log.Info("In IsOpenshift => returning true, nil")
				return true, nil
			}
	}
	return false, nil
}