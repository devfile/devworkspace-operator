package utils

import (
	"fmt"
	"strconv"

	"github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/config"
)

// IngressHostname generates a hostname based on service name and namespace
func IngressHostname(serviceName, namespace string, port int64) string {
	ingressName := IngressName(serviceName, port)
	return fmt.Sprintf("%s-%s.%s", ingressName, namespace, config.ControllerCfg.GetIngressGlobalDomain())
}

// IngressName genereates a names for ingresses
func IngressName(serviceName string, port int64) string {
	portString := strconv.FormatInt(port, 10)
	return serviceName + "-" + portString
}
