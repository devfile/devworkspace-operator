package model

import (
	corev1 "k8s.io/api/core/v1"
)

var (
	DefaultApiEndpoint        = "http://localhost:9999/api"
	CheOriginalName           = "workspace"
	AuthEnabled               = "false"
	ServicePortProtocol       = corev1.ProtocolTCP
	ServiceAccount            = "che-workspace"
	SidecarDefaultMemoryLimit = "128M"
	PVCStorageSize            = "1Gi"
	CheVersion                = "7.1.0"
)
