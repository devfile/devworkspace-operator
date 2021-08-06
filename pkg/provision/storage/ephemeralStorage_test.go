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

package storage

import (
	"testing"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/stretchr/testify/assert"

	wsprovision "github.com/devfile/devworkspace-operator/pkg/provision/workspace"
)

func TestRewriteContainerVolumeMountsForEphemeralStorageClass(t *testing.T) {
	tests := loadAllTestCasesOrPanic(t, "testdata/ephemeral-storage")
	setupControllerCfg()
	commonStorage := EphemeralStorageProvisioner{}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// sanity check that file is read correctly.
			assert.NotNil(t, tt.Input.Workspace, "Input does not define workspace")
			workspace := &dw.DevWorkspace{}
			workspace.Spec.Template = *tt.Input.Workspace
			workspace.Status.DevWorkspaceId = tt.Input.DevWorkspaceID
			workspace.Namespace = "test-namespace"
			err := commonStorage.ProvisionStorage(&tt.Input.PodAdditions, workspace, wsprovision.ClusterAPI{})
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
