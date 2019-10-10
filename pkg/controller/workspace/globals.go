package workspace

import (
	corev1 "k8s.io/api/core/v1"
)

var (
	defaultImagePullPolicy    = corev1.PullAlways
	defaultApiEndpoint        = "http://localhost:9999/api"
	cheOriginalName           = "workspace"
	authEnabled               = "false"
	servicePortProtocol       = corev1.ProtocolTCP
	serviceAccount            = "che-workspace"
	sidecarDefaultMemoryLimit = "128M"
	pvcStorageSize            = "1Gi"
)
