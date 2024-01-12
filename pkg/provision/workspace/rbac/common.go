// Copyright (c) 2019-2024 Red Hat, Inc.
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

package rbac

import (
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
)

var rbacLabels = map[string]string{
	"app.kubernetes.io/name":               "devworkspace-workspaces",
	"app.kubernetes.io/part-of":            "devworkspace-operator",
	"controller.devfile.io/workspace-rbac": "true",
}

func SyncRBAC(workspace *common.DevWorkspaceWithConfig, api sync.ClusterAPI) error {
	if err := cleanupDeprecatedRBAC(workspace.Namespace, api); err != nil {
		return err
	}
	if err := syncRoles(workspace, api); err != nil {
		return err
	}
	if err := syncRolebindings(workspace, api); err != nil {
		return err
	}
	return nil
}
