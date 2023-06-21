//
// Copyright (c) 2019-2023 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

// Package constants defines constant values used throughout the DevWorkspace Operator
package constants

import "k8s.io/apimachinery/pkg/util/intstr"

// Labels which should be used for controller related objects
var ControllerAppLabels = func() map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":    "devworkspace-controller",
		"app.kubernetes.io/part-of": "devworkspace-operator",
	}
}

var (
	// Maximum number of unavailable workspace pods when using the RollingUpdate deployment strategy
	RollingUpdateMaxUnavailable = intstr.FromInt(0)
	// Maximum number of excesss workspace pods when using the RollingUpdate deployment strategy
	RollingUpdateMaximumSurge = intstr.FromInt(1)
)

// Internal constants
const (
	DefaultProjectsSourcesRoot = "/projects"

	HomeUserDirectory = "/home/user/"

	HomeVolumeName = "persistentHome"

	ServiceAccount = "devworkspace"

	PVCStorageSize = "10Gi"

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

	// Constants describing storage classes supported by the controller

	// CommonStorageClassType defines the 'common' storage policy, which is an alias of the 'per-user' storage policy, and operates in the same fashion as the 'per-user' storage policy.
	// The 'common' storage policy exists only for legacy compatibility.
	CommonStorageClassType = "common"
	// PerUserStorageClassType defines the 'per-user' storage policy -- one PVC is provisioned per namespace and all devworkspace storage
	// is mounted in it on subpaths according to devworkspace ID.
	PerUserStorageClassType = "per-user"
	// AsyncStorageClassType defines the 'asynchronous' storage policy. An rsync sidecar is added to devworkspaces that uses SSH to connect
	// to a storage deployment that mounts a common PVC for the namespace.
	AsyncStorageClassType = "async"
	// EphemeralStorageClassType defines the 'ephemeral' storage policy: all volumes are allocated as emptyDir volumes and
	// so do not require cleanup. When a DevWorkspace is stopped, all local changes are lost.
	EphemeralStorageClassType = "ephemeral"
	// PerWorkspaceStorageClassType defines the 'per-workspace' storage policy: a PVC is provisioned for each workspace within the namespace.
	// All of the workspace's storage (volume mounts) are mounted on subpaths within the workspace's PVC.
	PerWorkspaceStorageClassType = "per-workspace"

	// CheCommonPVCName is the name of the common PVC equivalent used by Che. If present in the namespace, this PVC is mounted instead
	// of the default PVC when the 'common' or 'async' storage classes are used.
	CheCommonPVCName = "claim-che-workspace"

	// Constants describing configuration for automatic project cloning

	// ProjectCloneDisable specifies that project cloning should be disabled.
	ProjectCloneDisable = "disable"
)
