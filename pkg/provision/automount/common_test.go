//
// Copyright (c) 2019-2022 Red Hat, Inc.
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

package automount

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
)

type mountedVolumeType int

const (
	devWorkspaceVolume mountedVolumeType = iota
	secretVolumeType
	configMapVolumeType
)

const testContainerName = "testContainer"

func TestCheckAutoMountVolumesForCollision(t *testing.T) {
	type volumeDesc struct {
		name       string
		mountPath  string
		volumeType mountedVolumeType
	}
	tests := []struct {
		name                  string
		basePodAdditions      []volumeDesc
		automountPodAdditions []volumeDesc
		errRegexp             string
	}{
		{
			name: "Does not error when mounts are valid",
			basePodAdditions: []volumeDesc{
				{
					name:       "baseVolume",
					mountPath:  "basePath",
					volumeType: configMapVolumeType,
				},
			},
			automountPodAdditions: []volumeDesc{
				{
					name:       "automountConfigMap",
					mountPath:  "/configmap/mount",
					volumeType: configMapVolumeType,
				},
				{
					name:       "automountSecret",
					mountPath:  "/secret/mount",
					volumeType: secretVolumeType,
				},
			},
		},
		{
			name: "Detects volume name collision",
			basePodAdditions: []volumeDesc{
				{
					name:       "baseVolume",
					mountPath:  "basePath",
					volumeType: devWorkspaceVolume,
				},
			},
			automountPodAdditions: []volumeDesc{
				{
					name:       "baseVolume",
					mountPath:  "/configmap/mount",
					volumeType: configMapVolumeType,
				},
			},
			errRegexp: "DevWorkspace volume 'baseVolume' conflicts with automounted volume from configmap 'baseVolume'",
		},
		{
			name: "Detects mountPath collision with DevWorkspace",
			basePodAdditions: []volumeDesc{
				{
					name:       "baseVolume",
					mountPath:  "/collision/path",
					volumeType: devWorkspaceVolume,
				},
			},
			automountPodAdditions: []volumeDesc{
				{
					name:       "testVolume",
					mountPath:  "/collision/path",
					volumeType: secretVolumeType,
				},
			},
			errRegexp: fmt.Sprintf("DevWorkspace volume 'baseVolume' in container %s has same mountpath as auto-mounted volume from secret 'testVolume'", testContainerName),
		},
		{
			name: "Detects mountPath collision in automounted volumes",
			automountPodAdditions: []volumeDesc{
				{
					name:       "testVolume1",
					mountPath:  "/test/mount",
					volumeType: secretVolumeType,
				},
				{
					name:       "testVolume2",
					mountPath:  "/test/mount",
					volumeType: configMapVolumeType,
				},
			},
			errRegexp: "auto-mounted volumes from configmap 'testVolume2' and secret 'testVolume1' have the same mount path",
		},
	}

	convertDescToVolume := func(desc volumeDesc) (*corev1.Volume, *corev1.VolumeMount, *corev1.Container) {
		switch desc.volumeType {
		case secretVolumeType:
			volume := &corev1.Volume{
				Name: desc.name,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: desc.name,
					},
				},
			}
			volumeMount := &corev1.VolumeMount{
				Name:      desc.name,
				MountPath: desc.mountPath,
			}
			return volume, volumeMount, nil
		case configMapVolumeType:
			volume := &corev1.Volume{
				Name: desc.name,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: desc.name,
						},
					},
				},
			}
			volumeMount := &corev1.VolumeMount{
				Name:      desc.name,
				MountPath: desc.mountPath,
			}
			return volume, volumeMount, nil
		case devWorkspaceVolume:
			volume := &corev1.Volume{
				Name: desc.name,
			}
			container := &corev1.Container{
				Name: testContainerName,
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      desc.name,
						MountPath: desc.mountPath,
					},
				},
			}
			return volume, nil, container
		}
		return nil, nil, nil
	}

	convertToPodAddition := func(descs ...volumeDesc) *v1alpha1.PodAdditions {
		pa := &v1alpha1.PodAdditions{}
		for _, desc := range descs {
			volume, volumeMount, container := convertDescToVolume(desc)
			if volume != nil {
				pa.Volumes = append(pa.Volumes, *volume)
			}
			if volumeMount != nil {
				pa.VolumeMounts = append(pa.VolumeMounts, *volumeMount)
			}
			if container != nil {
				pa.Containers = append(pa.Containers, *container)
			}
		}
		return pa
	}

	convertToAutomountResources := func(descs ...volumeDesc) *Resources {
		resources := &Resources{}
		for _, desc := range descs {
			volume, volumeMount, _ := convertDescToVolume(desc)
			if volume != nil {
				resources.Volumes = append(resources.Volumes, *volume)
			}
			if volumeMount != nil {
				resources.VolumeMounts = append(resources.VolumeMounts, *volumeMount)
			}
		}
		return resources
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			base := convertToPodAddition(tt.basePodAdditions...)

			autoMount := convertToAutomountResources(tt.automountPodAdditions...)

			outErr := checkAutomountVolumesForCollision(base, autoMount)
			if tt.errRegexp == "" {
				assert.Nil(t, outErr, "Expected no error but got %s", outErr)
			} else {
				assert.NotNil(t, outErr, "Expected error but got nil")
				assert.Regexp(t, tt.errRegexp, outErr, "Error message should match regexp %s", tt.errRegexp)
			}
		})
	}
}
