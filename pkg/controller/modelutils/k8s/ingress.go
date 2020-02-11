package utils

import "github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/model"
import "github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/config"

func IngressHostName(name string, wkspCtx model.WorkspaceContext) string {
	return name + "-" + wkspCtx.Namespace + "." + config.ControllerCfg.GetIngressGlobalDomain()
}
