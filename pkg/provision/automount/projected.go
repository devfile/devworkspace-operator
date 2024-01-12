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
	"fmt"
	"sort"
	"strings"

	"github.com/devfile/devworkspace-operator/pkg/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

// mergeProjectedVolumes merges secret and configmap automount resources that share a mount path
// into projected volumes.
func mergeProjectedVolumes(resources *Resources) (*Resources, error) {
	mergedResources := &Resources{}

	// Map of mountPath -> volumeMount, to detect colliding volumeMounts
	mountPathToVolumeMounts := map[string][]corev1.VolumeMount{}
	needsProjectedVolume := false
	// Ordered list of mountPaths to process, to avoid random iteration order on maps
	var mountPathOrder []string
	for _, volumeMount := range resources.VolumeMounts {
		if len(mountPathToVolumeMounts[volumeMount.MountPath]) == 0 {
			mountPathOrder = append(mountPathOrder, volumeMount.MountPath)
		} else {
			needsProjectedVolume = true
		}
		mountPathToVolumeMounts[volumeMount.MountPath] = append(mountPathToVolumeMounts[volumeMount.MountPath], volumeMount)
	}
	if !needsProjectedVolume {
		// Return early and do nothing if we didn't find a mountPath collision above
		return resources, nil
	}
	sort.Strings(mountPathOrder)

	// Map of volume names -> volumes, for easier lookup
	volumeNameToVolume := map[string]corev1.Volume{}
	for _, volume := range resources.Volumes {
		volumeNameToVolume[volume.Name] = volume
	}

	for _, mountPath := range mountPathOrder {
		volumeMounts := mountPathToVolumeMounts[mountPath]
		switch len(volumeMounts) {
		case 0:
			continue
		case 1:
			// No projected volume necessary
			mergedResources.VolumeMounts = append(mergedResources.VolumeMounts, volumeMounts[0])
			volume := volumeNameToVolume[volumeMounts[0].Name]
			mergedResources.Volumes = append(mergedResources.Volumes, volume)
		default:
			vm, vol, err := generateProjectedVolume(mountPath, volumeMounts, volumeNameToVolume)
			if err != nil {
				return nil, err
			}
			mergedResources.VolumeMounts = append(mergedResources.VolumeMounts, *vm)
			mergedResources.Volumes = append(mergedResources.Volumes, *vol)
		}
	}

	mergedResources.EnvFromSource = resources.EnvFromSource

	return mergedResources, nil
}

// generateProjectedVolume creates a projected Volume and VolumeMount that should be used in place multiple VolumeMounts
// with the same mountPath.
func generateProjectedVolume(mountPath string, volumeMounts []corev1.VolumeMount, volumeNameToVolume map[string]corev1.Volume) (*corev1.VolumeMount, *corev1.Volume, error) {
	volumeName := common.AutoMountProjectedVolumeName(mountPath)
	projectedVolume := &corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			Projected: &corev1.ProjectedVolumeSource{
				DefaultMode: pointer.Int32(0640),
			},
		},
	}

	for _, vm := range volumeMounts {
		if err := checkCanUseProjectedVolumes(volumeMounts, volumeNameToVolume); err != nil {
			return nil, nil, err
		}

		volume := volumeNameToVolume[vm.Name]
		projection := corev1.VolumeProjection{}
		switch {
		case volume.Secret != nil:
			projection.Secret = &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: volume.Secret.SecretName,
				},
				Items: volume.Secret.Items,
			}
		case volume.ConfigMap != nil:
			projection.ConfigMap = &corev1.ConfigMapProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: volume.ConfigMap.Name,
				},
				Items: volume.ConfigMap.Items,
			}
		default:
			return nil, nil, fmt.Errorf("unrecognized volume type for volume %s", volume.Name)
		}
		projectedVolume.Projected.Sources = append(projectedVolume.Projected.Sources, projection)
	}

	// Order of resources here may be random; to avoid unnecessarily updating deployment
	// we need to sort them somehow.
	sort.Slice(projectedVolume.Projected.Sources, func(i, j int) bool {
		iSource, jSource := projectedVolume.Projected.Sources[i], projectedVolume.Projected.Sources[j]
		switch {
		case iSource.ConfigMap != nil && jSource.ConfigMap == nil:
			return true // ConfigMaps first
		case iSource.ConfigMap == nil && jSource.ConfigMap != nil:
			return false
		case iSource.ConfigMap != nil && jSource.ConfigMap != nil:
			return iSource.ConfigMap.Name < jSource.ConfigMap.Name
		default: // both sources are Secrets
			return iSource.Secret.Name < jSource.Secret.Name
		}
	})

	projectedVolumeMount := &corev1.VolumeMount{
		Name:      volumeName,
		MountPath: mountPath,
		ReadOnly:  true,
	}

	return projectedVolumeMount, projectedVolume, nil
}

// checkCanProjectedVolumes checks whether a set of volumeMounts (assumed to share the same mountPath) can be merged
// into a single VolumeMount and projected Volume. Returns an error if VolumeMounts should not be merged.
func checkCanUseProjectedVolumes(volumeMounts []corev1.VolumeMount, volumeNameToVolume map[string]corev1.Volume) error {
	isError := false
	for _, vm := range volumeMounts {
		// If any of the volumeMounts is using a subPath (and mountPaths collide) this is not an issue we can fix. This shouldn't
		// happen often with automount volumes as it would require e.g. two configmaps with the same mountPath and key
		if vm.SubPath != "" || vm.SubPathExpr != "" {
			isError = true
		}
		vol := volumeNameToVolume[vm.Name]
		if vol.PersistentVolumeClaim != nil {
			isError = true
		}
	}
	if isError {
		var problemNames []string
		for _, vm := range volumeMounts {
			problemNames = append(problemNames, formatVolumeDescription(volumeNameToVolume[vm.Name]))
		}
		return fmt.Errorf("auto-mounted volumes from (%s) have the same mount path", strings.Join(problemNames, ", "))
	}
	return nil
}
