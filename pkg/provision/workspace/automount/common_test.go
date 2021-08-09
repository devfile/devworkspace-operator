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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			convertToPodAddition := func(desc volumeDesc) v1alpha1.PodAdditions {
				pa := v1alpha1.PodAdditions{}
				switch desc.volumeType {
				case secretVolumeType:
					pa.Volumes = append(pa.Volumes, corev1.Volume{
						Name: desc.name,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: desc.name,
							},
						},
					})
				case configMapVolumeType:
					pa.Volumes = append(pa.Volumes, corev1.Volume{
						Name: desc.name,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: desc.name,
								},
							},
						},
					})
				case devWorkspaceVolume:
					pa.Volumes = append(pa.Volumes, corev1.Volume{
						Name: desc.name,
					})
				}
				switch desc.volumeType {
				case devWorkspaceVolume:
					container := corev1.Container{
						Name: testContainerName,
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      desc.name,
								MountPath: desc.mountPath,
							},
						},
					}
					pa.Containers = append(pa.Containers, container)
				case secretVolumeType, configMapVolumeType:
					pa.VolumeMounts = append(pa.VolumeMounts, corev1.VolumeMount{
						Name:      desc.name,
						MountPath: desc.mountPath,
					})
				}

				return pa
			}
			var base []v1alpha1.PodAdditions
			for _, desc := range tt.basePodAdditions {
				base = append(base, convertToPodAddition(desc))
			}
			var automount []v1alpha1.PodAdditions
			for _, desc := range tt.automountPodAdditions {
				automount = append(automount, convertToPodAddition(desc))
			}
			outErr := CheckAutoMountVolumesForCollision(base, automount)
			if tt.errRegexp == "" {
				assert.Nil(t, outErr, "Expected no error but got %s", outErr)
			} else {
				assert.NotNil(t, outErr, "Expected error but got nil")
				assert.Regexp(t, tt.errRegexp, outErr, "Error message should match regexp %s", tt.errRegexp)
			}
		})
	}
}
