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

package automount

import (
	"path"
	"sort"
	"strings"
	"testing"

	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

var resourcesDiffOpts = cmp.Options{
	cmpopts.SortSlices(func(a, b corev1.Volume) bool {
		return strings.Compare(a.Name, b.Name) > 0
	}),
	cmpopts.SortSlices(func(a, b corev1.VolumeMount) bool {
		return strings.Compare(a.Name, b.Name) > 0
	}),
	cmpopts.SortSlices(func(a, b corev1.EnvFromSource) bool {
		var aName, bName string
		if a.ConfigMapRef != nil {
			aName = a.ConfigMapRef.Name
		} else {
			aName = a.SecretRef.Name
		}
		if b.ConfigMapRef != nil {
			bName = b.ConfigMapRef.Name
		} else {
			bName = b.SecretRef.Name
		}

		return strings.Compare(aName, bName) > 0
	}),
}

func TestMergeProjectedVolumes(t *testing.T) {
	tests := []struct {
		name              string
		inputResources    *Resources
		expectedResources *Resources
		errRegexp         string
	}{
		{
			name: "No merging necessary",
			inputResources: &Resources{
				Volumes: []corev1.Volume{
					testSecretVolume("test-secret"),
					testConfigmapVolume("test-configmap"),
					testPVCVolume("test-pvc"),
				},
				VolumeMounts: []corev1.VolumeMount{
					testVolumeMount("test-secret", "test-secret"),
					testVolumeMount("test-configmap", "test-configmap"),
					testVolumeMount("test-pvc", "test-pvc"),
				},
				EnvFromSource: []corev1.EnvFromSource{
					testEnvFromSource("test-envfrom"),
				},
			},
			expectedResources: &Resources{
				Volumes: []corev1.Volume{
					testSecretVolume("test-secret"),
					testConfigmapVolume("test-configmap"),
					testPVCVolume("test-pvc"),
				},
				VolumeMounts: []corev1.VolumeMount{
					testVolumeMount("test-secret", "test-secret"),
					testVolumeMount("test-configmap", "test-configmap"),
					testVolumeMount("test-pvc", "test-pvc"),
				},
				EnvFromSource: []corev1.EnvFromSource{
					testEnvFromSource("test-envfrom"),
				},
			},
		},
		{
			name: "Merges configmap and secret volume",
			inputResources: &Resources{
				Volumes: []corev1.Volume{
					testSecretVolume("test-secret"),
					testConfigmapVolume("test-configmap"),
					testPVCVolume("test-pvc"),
				},
				VolumeMounts: []corev1.VolumeMount{
					testVolumeMount("test-secret", "test-path"),
					testVolumeMount("test-configmap", "test-path"),
					testVolumeMount("test-pvc", "test-pvc"),
				},
				EnvFromSource: []corev1.EnvFromSource{
					testEnvFromSource("test-envfrom"),
				},
			},
			expectedResources: &Resources{
				Volumes: []corev1.Volume{
					testProjectedVolume(common.AutoMountProjectedVolumeName("test-path"), []string{"test-secret"}, []string{"test-configmap"}),
					testPVCVolume("test-pvc"),
				},
				VolumeMounts: []corev1.VolumeMount{
					testVolumeMount(common.AutoMountProjectedVolumeName("test-path"), "test-path"),
					testVolumeMount("test-pvc", "test-pvc"),
				},
				EnvFromSource: []corev1.EnvFromSource{
					testEnvFromSource("test-envfrom"),
				},
			},
		},
		{
			name: "Merges only necessary volumes",
			inputResources: &Resources{
				Volumes: []corev1.Volume{
					testSecretVolume("test-secret"),
					testConfigmapVolume("test-configmap"),
					testConfigmapVolume("test-configmap-2"),
					testSecretVolume("test-unmerged-secret"),
					testConfigmapVolume("test-unmerged-configmap"),
					testPVCVolume("test-pvc"),
				},
				VolumeMounts: []corev1.VolumeMount{
					testVolumeMount("test-secret", "test-path"),
					testVolumeMount("test-configmap", "test-path"),
					testVolumeMount("test-configmap-2", "test-path"),
					testVolumeMount("test-unmerged-secret", "secret-path"),
					testVolumeMount("test-unmerged-configmap", "cm-path"),
					testVolumeMount("test-pvc", "test-pvc"),
				},
				EnvFromSource: []corev1.EnvFromSource{
					testEnvFromSource("test-envfrom"),
				},
			},
			expectedResources: &Resources{
				Volumes: []corev1.Volume{
					testProjectedVolume(common.AutoMountProjectedVolumeName("test-path"), []string{"test-secret"}, []string{"test-configmap", "test-configmap-2"}),
					testSecretVolume("test-unmerged-secret"),
					testConfigmapVolume("test-unmerged-configmap"),
					testPVCVolume("test-pvc"),
				},
				VolumeMounts: []corev1.VolumeMount{
					testVolumeMount(common.AutoMountProjectedVolumeName("test-path"), "test-path"),
					testVolumeMount("test-unmerged-secret", "secret-path"),
					testVolumeMount("test-unmerged-configmap", "cm-path"),
					testVolumeMount("test-pvc", "test-pvc"),
				},
				EnvFromSource: []corev1.EnvFromSource{
					testEnvFromSource("test-envfrom"),
				},
			},
		},
		{
			name: "Error when would merge subpath volumeMount",
			inputResources: &Resources{
				Volumes: []corev1.Volume{
					testSecretVolume("test-secret"),
					testConfigmapVolume("test-configmap"),
				},
				VolumeMounts: []corev1.VolumeMount{
					testVolumeMount("test-secret", "test-path/subpath"),
					testSubpathVolumeMount("test-configmap", "test-path", "subpath"),
				},
				EnvFromSource: []corev1.EnvFromSource{
					testEnvFromSource("test-envfrom"),
				},
			},
			errRegexp: `auto-mounted volumes from \(secret 'test-secret', configmap 'test-configmap'\) have the same mount path`,
		},
		{
			name: "Error when unrecognized volume type",
			inputResources: &Resources{
				Volumes: []corev1.Volume{
					{
						Name: "unrecognized-volume",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{Path: "test"},
						},
					},
					testConfigmapVolume("test-cm"),
				},
				VolumeMounts: []corev1.VolumeMount{
					testVolumeMount("unrecognized-volume", "test"),
					testVolumeMount("test-cm", "test"),
				},
			},
			errRegexp: `unrecognized volume type for volume unrecognized-volume`,
		},
		{
			name: "Error when trying to merge PVC and configmap",
			inputResources: &Resources{
				Volumes: []corev1.Volume{
					testSecretVolume("test-secret"),
					testConfigmapVolume("test-configmap"),
					testPVCVolume("test-pvc"),
				},
				VolumeMounts: []corev1.VolumeMount{
					testVolumeMount("test-secret", "test-path"),
					testVolumeMount("test-configmap", "test-path"),
					testVolumeMount("test-pvc", "test-path"),
				},
				EnvFromSource: []corev1.EnvFromSource{
					testEnvFromSource("test-envfrom"),
				},
			},
			errRegexp: `auto-mounted volumes from \(secret 'test-secret', configmap 'test-configmap', pvc 'test-pvc'\) have the same mount path`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualResources, err := mergeProjectedVolumes(tt.inputResources)
			if tt.errRegexp != "" {
				if !assert.Error(t, err) {
					return
				}
				assert.Regexp(t, tt.errRegexp, err.Error(), "Error message should match")
			} else {
				if !assert.NoError(t, err, "Should not return error") {
					return
				}
				assert.True(t, cmp.Equal(tt.expectedResources.Volumes, actualResources.Volumes, resourcesDiffOpts), cmp.Diff(tt.expectedResources.Volumes, actualResources.Volumes, resourcesDiffOpts))
				assert.True(t, cmp.Equal(tt.expectedResources.VolumeMounts, actualResources.VolumeMounts, resourcesDiffOpts), cmp.Diff(tt.expectedResources.VolumeMounts, actualResources.VolumeMounts, resourcesDiffOpts))
				assert.True(t, cmp.Equal(tt.expectedResources.EnvFromSource, actualResources.EnvFromSource, resourcesDiffOpts), cmp.Diff(tt.expectedResources.EnvFromSource, actualResources.EnvFromSource, resourcesDiffOpts))
			}
		})
	}
}

func testSecretVolume(name string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: name,
			},
		},
	}
}

func testConfigmapVolume(name string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: name,
				},
			},
		},
	}
}

func testPVCVolume(name string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: name,
			},
		},
	}
}

func testVolumeMount(name, mountPath string) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      name,
		ReadOnly:  true,
		MountPath: mountPath,
	}
}

func testSubpathVolumeMount(name, mountPath, subpath string) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      name,
		ReadOnly:  true,
		MountPath: path.Join(mountPath, subpath),
		SubPath:   subpath,
	}
}

func testEnvFromSource(name string) corev1.EnvFromSource {
	return corev1.EnvFromSource{
		ConfigMapRef: &corev1.ConfigMapEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: name,
			},
		},
	}
}

func testProjectedVolume(name string, secretNames, configmapNames []string) corev1.Volume {
	vol := corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			Projected: &corev1.ProjectedVolumeSource{
				DefaultMode: pointer.Int32(0640),
			},
		},
	}
	sort.Strings(configmapNames)
	sort.Strings(secretNames)
	for _, configmapName := range configmapNames {
		vol.Projected.Sources = append(vol.Projected.Sources, corev1.VolumeProjection{
			ConfigMap: &corev1.ConfigMapProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configmapName,
				},
			},
		})
	}
	for _, secretName := range secretNames {
		vol.Projected.Sources = append(vol.Projected.Sources, corev1.VolumeProjection{
			Secret: &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: secretName,
				},
			},
		})
	}

	return vol
}
