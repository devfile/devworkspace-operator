package config

const (
	DefaultServerImageName   = "quay.io/che-incubator/che-workspace-crd-rest-apis:7.1.0"
	DefaultSidecarPullPolicy = "Always"

	DefaultInitPluginBrokerImage    = "eclipse/che-init-plugin-broker:v0.20"
	DefaultUnifiedPluginBrokerImage = "eclipse/che-unified-plugin-broker:v0.20"

	DefaultIngressGlobalDomain = ""

	DefaultWorkspacePVCName = "claim-che-workspace"
)
