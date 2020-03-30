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

	pluginArtifactsBrokerImage        = "che.workspace.plugin_broker.artifacts.image"
	defaultPluginArtifactsBrokerImage = "quay.io/eclipse/che-plugin-artifacts-broker:v3.1.0"

	//routingClass defines the default routing class that should be used if user does not specify it explicitly
	routingClass        = "che.default_routing_class"
	defaultRoutingClass = "basic"

	webhooksEnabled        = "che.webhooks.enabled"
	defaultWebhooksEnabled = "false"
)
