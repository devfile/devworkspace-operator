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

package workspace

import (
	"fmt"
	"sort"
	"strings"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	"github.com/devfile/devworkspace-operator/pkg/common"
)

func ProvisionServiceAccountTokensInto(podAdditions *v1alpha1.PodAdditions, workspace *common.DevWorkspaceWithConfig) error {
	saTokenVolumeMounts, saTokenVolumes, err := getSATokensVolumesAndVolumeMounts(workspace.Config.Workspace.ServiceAccount.ServiceAccountTokens)
	if err != nil {
		return err
	}
	podAdditions.VolumeMounts = append(podAdditions.VolumeMounts, saTokenVolumeMounts...)
	podAdditions.Volumes = append(podAdditions.Volumes, saTokenVolumes...)
	return nil
}

// Returns VolumeMounts and (projected) Volumes corresponding to the provided ServiceAccount tokens.
// An error is returned if at least two ServiceAccount tokens share the same mounth path and path.
func getSATokensVolumesAndVolumeMounts(serviceAccountTokens []v1alpha1.ServiceAccountToken) ([]corev1.VolumeMount, []corev1.Volume, error) {
	volumeMounts := []corev1.VolumeMount{}
	volumes := []corev1.Volume{}
	// Map of mountPath -> ServiceAccountTokens, to detect colliding volumeMounts
	mountPathToSATokens := map[string][]v1alpha1.ServiceAccountToken{}

	// Ordered list of mountPaths to process, to avoid random iteration order on maps
	var mountPathOrder []string
	for _, saToken := range serviceAccountTokens {
		if len(mountPathToSATokens[saToken.MountPath]) == 0 {
			mountPathOrder = append(mountPathOrder, saToken.MountPath)
		}
		mountPathToSATokens[saToken.MountPath] = append(mountPathToSATokens[saToken.MountPath], saToken)
	}
	sort.Strings(mountPathOrder)

	for _, mountPath := range mountPathOrder {
		saTokens := mountPathToSATokens[mountPath]
		vm, vol, err := generateSATokenProjectedVolume(mountPath, saTokens)
		if err != nil {
			return nil, nil, err
		}
		volumeMounts = append(volumeMounts, *vm)
		volumes = append(volumes, *vol)
	}
	return volumeMounts, volumes, nil
}

// Returns a VolumeMount and projected Volume for the provided ServiceAccount tokens that share the same mount path.
// The VolumeMount's mount path is set to the provided mount path and its name is set to the ServiceAccount token's name
// if only a single ServiceAccount token is provided, otherwise a common VolumeMount name is used
// if multiple ServiceAccount token's are provided.
//
// An error is returned if at least two ServiceAccount tokens share the same mount path and volume path
// (as this would lead to one of the tokens overwriting the others).
func generateSATokenProjectedVolume(mountPath string, saTokens []v1alpha1.ServiceAccountToken) (*corev1.VolumeMount, *corev1.Volume, error) {
	// Check if two tokens share the same path and mountPath, which is invalid
	err := checkSATokenPathCollisions(saTokens, mountPath)
	if err != nil {
		return nil, nil, err
	}

	var volumeName string
	if len(saTokens) == 1 {
		volumeName = saTokens[0].Name
	} else {
		// If multiple tokens are being projected into the same mount path, use a common name
		volumeName = common.ServiceAccountTokenProjectionName(mountPath)
	}

	volume := &corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			Projected: &corev1.ProjectedVolumeSource{
				DefaultMode: pointer.Int32(0640),
			},
		},
	}

	for _, saToken := range saTokens {
		volumeProjection := &corev1.VolumeProjection{
			ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
				Audience:          saToken.Audience,
				ExpirationSeconds: pointer.Int64(saToken.ExpirationSeconds),
				Path:              saToken.Path,
			},
		}
		volume.Projected.Sources = append(volume.Projected.Sources, *volumeProjection)
	}

	volumeMount := &corev1.VolumeMount{
		Name:      volumeName,
		MountPath: mountPath,
		ReadOnly:  true,
	}

	return volumeMount, volume, nil
}

// Checks if any of the given ServiceAccount tokens (which are assumed to share the same mount path)
// have the same volume path, which would result in a mounting collision (i.e. one of the tokens would overwrite the others).
// Returns an error if at least two ServiceAccount tokens share the same volume path and mount path or nil otherwise
func checkSATokenPathCollisions(saTokens []v1alpha1.ServiceAccountToken, mountPath string) error {
	pathsToSATokens := map[string][]v1alpha1.ServiceAccountToken{}
	isError := false
	problemPaths := map[string]bool{}
	for _, saToken := range saTokens {
		if len(pathsToSATokens[saToken.Path]) > 0 {
			isError = true
			problemPaths[saToken.Path] = true
		}
		pathsToSATokens[saToken.Path] = append(pathsToSATokens[saToken.Path], saToken)
	}

	if isError {
		var problemNames []string
		for path := range problemPaths {
			collidingSATokens := pathsToSATokens[path]
			for _, saToken := range collidingSATokens {
				problemNames = append(problemNames, saToken.Name)
			}
			if len(problemPaths) == 1 {
				sort.Strings(problemNames)
				return fmt.Errorf("the following ServiceAccount tokens have the same path (%s) and mount path (%s): %s", path, mountPath, strings.Join(problemNames, ", "))
			}
		}
		sort.Strings(problemNames)
		return fmt.Errorf("multiple ServiceAccount tokens share the same path and mount path: %s", strings.Join(problemNames, ", "))
	}
	return nil
}
