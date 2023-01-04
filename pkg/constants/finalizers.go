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

package constants

const (
	// StorageCleanupFinalizer is used to block DevWorkspace deletion when it is necessary
	// to clean up persistent storage used for the workspace.
	StorageCleanupFinalizer = "storage.controller.devfile.io"
	// ServiceAccountCleanupFinalizer is used to block DevWorkspace deletion when it is
	// necessary to clean up additional non-workspace roles added to the workspace
	// serviceaccount
	//
	// Deprecated: Will not be added to new workspaces but needs to be tracked for
	// removal to ensure workspaces that used it previously will be cleaned up.
	ServiceAccountCleanupFinalizer = "serviceaccount.controller.devfile.io"
	// RBACCleanupFinalizer is used to block DevWorkspace deletion in order to ensure
	// the workspace role and rolebinding are cleaned up correctly. Since each workspace
	// serviceaccount is added to the workspace rolebinding, it is necessary to remove it
	// when a workspace is deleted
	RBACCleanupFinalizer = "rbac.controller.devfile.io"
)
