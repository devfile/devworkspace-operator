package utils

import "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/model"
import "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/config"

func IngressHostName(name string, wkspProperties model.WorkspaceProperties) string {
	return name + "-" + wkspProperties.Namespace + "." + config.ControllerCfg.GetIngressGlobalDomain()
}
