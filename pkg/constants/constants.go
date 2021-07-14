//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

// package constants defines constant values used throughout the DevWorkspace Operator
package constants

// Labels which should be used for controller related objects
var ControllerAppLabels = func() map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":    "devworkspace-controller",
		"app.kubernetes.io/part-of": "devworkspace-operator",
	}
}

// Internal constants
const (
	DefaultProjectsSourcesRoot = "/projects"

	ServiceAccount = "devworkspace"

	SidecarDefaultMemoryLimit   = "128M"
	SidecarDefaultMemoryRequest = "64M"

	SidecarDefaultCpuLimit   = "" // do not provide any value
	SidecarDefaultCpuRequest = "" // do not provide any value

	PVCStorageSize = "1Gi"

	// DevWorkspaceIDLoggerKey is the key used to log workspace ID in the reconcile
	DevWorkspaceIDLoggerKey = "devworkspace_id"

	// ControllerServiceAccountNameEnvVar stores the name of the serviceaccount used in the controller.
	ControllerServiceAccountNameEnvVar = "CONTROLLER_SERVICE_ACCOUNT_NAME"

	// PVCCleanupPodMemoryLimit is the memory limit used for PVC clean up pods
	PVCCleanupPodMemoryLimit = "100Mi"

	// PVCCleanupPodMemoryRequest is the memory request used for PVC clean up pods
	PVCCleanupPodMemoryRequest = "32Mi"

	// PVCCleanupPodCPULimit is the cpu limit used for PVC clean up pods
	PVCCleanupPodCPULimit = "50m"

	// PVCCleanupPodCPURequest is the cpu request used for PVC clean up pods
	PVCCleanupPodCPURequest = "5m"

	// Resource limits/requests for project cloner init container
	ProjectCloneMemoryLimit   = "1Gi"
	ProjectCloneMemoryRequest = "128Mi"
	ProjectCloneCPULimit      = "1000m"
	ProjectCloneCPURequest    = "100m"

	// Constants describing storage classes supported by the controller
	// CommonStorageClassType defines the 'common' storage policy -- one PVC is provisioned per namespace and all devworkspace storage
	// is mounted in it on subpaths according to devworkspace ID.
	CommonStorageClassType = "common"
	// AsyncStorageClassType defines the 'asynchronous' storage policy. An rsync sidecar is added to devworkspaces that uses SSH to connect
	// to a storage deployment that mounts a common PVC for the namespace.
	AsyncStorageClassType = "async"
	// EphemeralStorageClassType defines the 'ephemeral' storage policy: all volumes are allocated as emptyDir volumes and
	// so do not require cleanup. When a DevWorkspace is stopped, all local changes are lost.
	EphemeralStorageClassType = "ephemeral"
)
