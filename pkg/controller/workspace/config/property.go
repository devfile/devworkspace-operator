package config

const (
	serverImageName        = "cherestapis.image.name"
	defaultServerImageName = "quay.io/che-incubator/che-workspace-crd-rest-apis:7.1.0"

	sidecarPullPolicy        = "sidecar.pull.policy"
	defaultSidecarPullPolicy = "Always"

	pluginRegistryURL = "plugin.registry.url"

	ingressGlobalDomain        = "ingress.global.domain"
	defaultIngressGlobalDomain = ""

	//workspacePVCName config property handles the PVC name that should be created and used for all workspaces within one kubernetes namespace
	workspacePVCName        = "pvc.name"
	defaultWorkspacePVCName = "claim-che-workspace"

	workspacePVCStorageClassName = "pvc.storage_class.name"

	unifiedPluginBrokerImage        = "che.workspace.plugin_broker.unified.image"
	defaultUnifiedPluginBrokerImage = "eclipse/che-unified-plugin-broker:v0.20"

	initPluginBrokerImage        = "che.workspace.plugin_broker.init.image"
	defaultInitPluginBrokerImage = "eclipse/che-init-plugin-broker:v0.20"
)
