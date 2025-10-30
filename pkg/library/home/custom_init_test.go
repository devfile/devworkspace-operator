//
// Copyright (c) 2019-2025 Red Hat, Inc.
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

package home

import (
	"testing"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	attributes "github.com/devfile/api/v2/pkg/attributes"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

func TestCustomInitPersistentHome(t *testing.T) {
	tests := []struct {
		name                    string
		workspace               *common.DevWorkspaceWithConfig
		expectDefaultInitAdded  bool
		expectCustomInitSkipped bool
	}{
		{
			name: "Skips default init when custom init-persistent-home is provided",
			workspace: &common.DevWorkspaceWithConfig{
				DevWorkspace: &dw.DevWorkspace{
					Spec: dw.DevWorkspaceSpec{
						Template: dw.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: dw.DevWorkspaceTemplateSpecContent{
								Components: []dw.Component{
									{
										Name: "test-container",
										ComponentUnion: dw.ComponentUnion{
											Container: &dw.ContainerComponent{
												Container: dw.Container{
													Image: "test-image:latest",
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Config: &v1alpha1.OperatorConfiguration{
					Workspace: &v1alpha1.WorkspaceConfig{
						PersistUserHome: &v1alpha1.PersistentHomeConfig{
							Enabled: pointer.Bool(true),
						},
						InitContainers: []corev1.Container{
							{
								Name: constants.HomeInitComponentName,
								Args: []string{"echo 'custom init'"},
							},
						},
					},
				},
			},
			expectDefaultInitAdded: false,
		},
		{
			name: "Adds default init when no custom init-persistent-home is provided",
			workspace: &common.DevWorkspaceWithConfig{
				DevWorkspace: &dw.DevWorkspace{
					Spec: dw.DevWorkspaceSpec{
						Template: dw.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: dw.DevWorkspaceTemplateSpecContent{
								Components: []dw.Component{
									{
										Name: "test-container",
										ComponentUnion: dw.ComponentUnion{
											Container: &dw.ContainerComponent{
												Container: dw.Container{
													Image: "test-image:latest",
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Config: &v1alpha1.OperatorConfiguration{
					Workspace: &v1alpha1.WorkspaceConfig{
						PersistUserHome: &v1alpha1.PersistentHomeConfig{
							Enabled: pointer.Bool(true),
						},
						InitContainers: []corev1.Container{
							{
								Name:  "custom-container",
								Image: "custom:latest",
								Args:  []string{"echo 'other init'"},
							},
						},
					},
				},
			},
			expectDefaultInitAdded: true,
		},
		{
			name: "Adds default init when custom init containers list is empty",
			workspace: &common.DevWorkspaceWithConfig{
				DevWorkspace: &dw.DevWorkspace{
					Spec: dw.DevWorkspaceSpec{
						Template: dw.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: dw.DevWorkspaceTemplateSpecContent{
								Components: []dw.Component{
									{
										Name: "test-container",
										ComponentUnion: dw.ComponentUnion{
											Container: &dw.ContainerComponent{
												Container: dw.Container{
													Image: "test-image:latest",
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Config: &v1alpha1.OperatorConfiguration{
					Workspace: &v1alpha1.WorkspaceConfig{
						PersistUserHome: &v1alpha1.PersistentHomeConfig{
							Enabled:              pointer.Bool(true),
							DisableInitContainer: pointer.Bool(false),
						},
						InitContainers: []corev1.Container{},
					},
				},
			},
			expectDefaultInitAdded: true,
		},
		{
			name: "Skips default init when DisableInitContainer is true even with custom init",
			workspace: &common.DevWorkspaceWithConfig{
				DevWorkspace: &dw.DevWorkspace{
					Spec: dw.DevWorkspaceSpec{
						Template: dw.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: dw.DevWorkspaceTemplateSpecContent{
								Components: []dw.Component{
									{
										Name: "test-container",
										ComponentUnion: dw.ComponentUnion{
											Container: &dw.ContainerComponent{
												Container: dw.Container{
													Image: "test-image:latest",
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Config: &v1alpha1.OperatorConfiguration{
					Workspace: &v1alpha1.WorkspaceConfig{
						PersistUserHome: &v1alpha1.PersistentHomeConfig{
							Enabled:              pointer.Bool(true),
							DisableInitContainer: pointer.Bool(true),
						},
					},
				},
			},
			expectDefaultInitAdded: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := AddPersistentHomeVolume(tt.workspace)
			assert.NoError(t, err)
			assert.NotNil(t, result)

			// Check if default init component was added
			hasDefaultInit := false
			for _, component := range result.Components {
				if component.Name == constants.HomeInitComponentName {
					hasDefaultInit = true
					break
				}
			}

			if tt.expectDefaultInitAdded {
				assert.True(t, hasDefaultInit, "Expected default init component to be added")
			} else {
				assert.False(t, hasDefaultInit, "Expected default init component NOT to be added")
			}

			// Verify persistent-home volume is always added
			hasPersistentHomeVolume := false
			for _, component := range result.Components {
				if component.Name == constants.HomeVolumeName {
					hasPersistentHomeVolume = true
					break
				}
			}
			assert.True(t, hasPersistentHomeVolume, "persistent-home volume should always be added")
		})
	}
}

func TestInferWorkspaceImage(t *testing.T) {
	tests := []struct {
		name          string
		template      *dw.DevWorkspaceTemplateSpec
		expectedImage string
	}{
		{
			name: "Returns first non-imported container image",
			template: &dw.DevWorkspaceTemplateSpec{
				DevWorkspaceTemplateSpecContent: dw.DevWorkspaceTemplateSpecContent{
					Components: []dw.Component{
						{
							Name: "main-container",
							ComponentUnion: dw.ComponentUnion{
								Container: &dw.ContainerComponent{
									Container: dw.Container{
										Image: "my-workspace:latest",
									},
								},
							},
						},
					},
				},
			},
			expectedImage: "my-workspace:latest",
		},
		{
			name: "Skips imported containers and returns first non-imported",
			template: &dw.DevWorkspaceTemplateSpec{
				DevWorkspaceTemplateSpecContent: dw.DevWorkspaceTemplateSpecContent{
					Components: []dw.Component{
						{
							Name: "plugin-container",
							Attributes: attributes.Attributes{}.
								PutString(constants.PluginSourceAttribute, "plugin-registry"),
							ComponentUnion: dw.ComponentUnion{
								Container: &dw.ContainerComponent{
									Container: dw.Container{
										Image: "plugin-image:latest",
									},
								},
							},
						},
						{
							Name: "main-container",
							ComponentUnion: dw.ComponentUnion{
								Container: &dw.ContainerComponent{
									Container: dw.Container{
										Image: "my-workspace:latest",
									},
								},
							},
						},
					},
				},
			},
			expectedImage: "my-workspace:latest",
		},
		{
			name: "Returns empty string when no suitable container found",
			template: &dw.DevWorkspaceTemplateSpec{
				DevWorkspaceTemplateSpecContent: dw.DevWorkspaceTemplateSpecContent{
					Components: []dw.Component{
						{
							Name: "volume",
							ComponentUnion: dw.ComponentUnion{
								Volume: &dw.VolumeComponent{},
							},
						},
					},
				},
			},
			expectedImage: "",
		},
		{
			name: "Returns empty string when all containers are imported",
			template: &dw.DevWorkspaceTemplateSpec{
				DevWorkspaceTemplateSpecContent: dw.DevWorkspaceTemplateSpecContent{
					Components: []dw.Component{
						{
							Name: "plugin-container",
							Attributes: attributes.Attributes{}.
								PutString(constants.PluginSourceAttribute, "plugin-registry"),
							ComponentUnion: dw.ComponentUnion{
								Container: &dw.ContainerComponent{
									Container: dw.Container{
										Image: "plugin-image:latest",
									},
								},
							},
						},
					},
				},
			},
			expectedImage: "",
		},
		{
			name: "Treats parent-sourced containers as non-imported",
			template: &dw.DevWorkspaceTemplateSpec{
				DevWorkspaceTemplateSpecContent: dw.DevWorkspaceTemplateSpecContent{
					Components: []dw.Component{
						{
							Name: "parent-container",
							Attributes: attributes.Attributes{}.
								PutString(constants.PluginSourceAttribute, "parent"),
							ComponentUnion: dw.ComponentUnion{
								Container: &dw.ContainerComponent{
									Container: dw.Container{
										Image: "parent-image:latest",
									},
								},
							},
						},
					},
				},
			},
			expectedImage: "parent-image:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := InferWorkspaceImage(tt.template)
			assert.Equal(t, tt.expectedImage, result)
		})
	}
}
