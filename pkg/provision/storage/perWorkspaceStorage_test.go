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
	"fmt"
	"testing"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	"github.com/google/go-cmp/cmp"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(dw.AddToScheme(scheme))
}

func TestRewriteContainerVolumeMountsForPerWorkspaceStorageClass(t *testing.T) {
	tests := loadAllTestCasesOrPanic(t, "testdata/perWorkspace-storage")
	perWorkspaceStorage := PerWorkspaceStorageProvisioner{}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// sanity check that file is read correctly.
			assert.NotNil(t, tt.Input.Workspace, "Input does not define workspace")
			workspace := getDevWorkspaceWithConfig(&dw.DevWorkspace{})
			workspace.Spec.Template = *tt.Input.Workspace
			workspace.Status.DevWorkspaceId = tt.Input.DevWorkspaceID
			workspace.Namespace = "test-namespace"

			clusterAPI := sync.ClusterAPI{
				Scheme: scheme,
				Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
				Logger: zap.New(),
			}

			if needsStorage(&workspace.Spec.Template) {
				err := perWorkspaceStorage.ProvisionStorage(tt.Input.PodAdditions.DeepCopy(), workspace, clusterAPI)
				if !assert.Error(t, err, "Should get a NotReady error when creating PVC") {
					return
				}

				assert.Regexp(t, err.Error(), fmt.Sprintf("Updated %s PVC on cluster", common.PerWorkspacePVCName(workspace.Status.DevWorkspaceId)))

				retrievedPVC := &corev1.PersistentVolumeClaim{}
				namespacedName := types.NamespacedName{Name: common.PerWorkspacePVCName(workspace.Status.DevWorkspaceId), Namespace: workspace.Namespace}

				err = clusterAPI.Client.Get(clusterAPI.Ctx, namespacedName, retrievedPVC)

				if !assert.NoError(t, err, "PVC should be created on cluster") {
					return
				}

				if !assert.NotEmpty(t, retrievedPVC.ObjectMeta.OwnerReferences) {
					return
				}
				assert.Len(t, retrievedPVC.ObjectMeta.OwnerReferences, 1)
				assert.Equal(t, retrievedPVC.ObjectMeta.OwnerReferences[0].Kind, "DevWorkspace")

				retrievedPVCSize := retrievedPVC.Spec.Resources.Requests[corev1.ResourceStorage]
				if tt.Output.PVCSize != nil {
					assert.True(t, tt.Output.PVCSize.Equal(retrievedPVCSize),
						"Calculated PVC size is incorrect, should be %s but got %s", tt.Output.PVCSize.String(), retrievedPVCSize.String())
				} else if tt.Output.ErrRegexp == nil {
					assert.True(t, workspace.Config.Workspace.DefaultStorageSize.PerWorkspace.Equal(retrievedPVCSize),
						"PVC size is incorrect, should use default PVC size of %s but got %s", workspace.Config.Workspace.DefaultStorageSize.PerWorkspace, retrievedPVCSize.String())
				}
			}

			actualPodAdditions := tt.Input.PodAdditions.DeepCopy()
			err := perWorkspaceStorage.ProvisionStorage(actualPodAdditions, workspace, clusterAPI)

			if tt.Output.ErrRegexp != nil && assert.Error(t, err) {
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error(), "Error message should match")
			} else {
				if !assert.NoError(t, err, "Should not return error") {
					return
				}

				sortVolumesAndVolumeMounts(&tt.Output.PodAdditions)
				sortVolumesAndVolumeMounts(actualPodAdditions)
				assert.Equal(t, tt.Output.PodAdditions, *actualPodAdditions,
					"PodAdditions should match expected output: Diff: %s", cmp.Diff(tt.Output.PodAdditions, *actualPodAdditions))
			}
		})
	}
}
