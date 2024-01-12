//
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
//

package storage

import (
	"testing"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	"github.com/stretchr/testify/assert"
)

func TestRewriteContainerVolumeMountsForEphemeralStorageClass(t *testing.T) {
	tests := loadAllTestCasesOrPanic(t, "testdata/ephemeral-storage")
	commonStorage := EphemeralStorageProvisioner{}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// sanity check that file is read correctly.
			assert.NotNil(t, tt.Input.Workspace, "Input does not define workspace")
			workspace := &dw.DevWorkspace{}
			workspace.Spec.Template = *tt.Input.Workspace
			workspace.Status.DevWorkspaceId = tt.Input.DevWorkspaceID
			workspace.Namespace = "test-namespace"
			err := commonStorage.ProvisionStorage(&tt.Input.PodAdditions, getDevWorkspaceWithConfig(workspace), sync.ClusterAPI{})
			if tt.Output.ErrRegexp != nil && assert.Error(t, err) {
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error(), "Error message should match")
			} else {
				if !assert.NoError(t, err, "Should not return error") {
					return
				}
				sortVolumesAndVolumeMounts(&tt.Output.PodAdditions)
				sortVolumesAndVolumeMounts(&tt.Input.PodAdditions)
				assert.Equal(t, tt.Output.PodAdditions, tt.Input.PodAdditions, "PodAdditions should match expected output")
			}
		})
	}
}
