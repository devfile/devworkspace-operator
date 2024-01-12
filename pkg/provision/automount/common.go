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

package automount

import (
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

var (
	defaultAccessMode = pointer.Int32(0640)
)

type Resources struct {
	Volumes       []corev1.Volume
	VolumeMounts  []corev1.VolumeMount
	EnvFromSource []corev1.EnvFromSource
}

func ProvisionAutoMountResourcesInto(podAdditions *v1alpha1.PodAdditions, api sync.ClusterAPI, namespace string) error {
	resources, err := getAutomountResources(api, namespace)

	if err != nil {
		return err
	}

	if err := checkAutomountVolumesForCollision(podAdditions, resources); err != nil {
		return err
	}

	for idx, container := range podAdditions.Containers {
		podAdditions.Containers[idx].VolumeMounts = append(container.VolumeMounts, resources.VolumeMounts...)
		podAdditions.Containers[idx].EnvFrom = append(container.EnvFrom, resources.EnvFromSource...)
	}

	for idx, initContainer := range podAdditions.InitContainers {
		podAdditions.InitContainers[idx].VolumeMounts = append(initContainer.VolumeMounts, resources.VolumeMounts...)
		podAdditions.InitContainers[idx].EnvFrom = append(initContainer.EnvFrom, resources.EnvFromSource...)
	}

	if resources.Volumes != nil {
		podAdditions.Volumes = append(podAdditions.Volumes, resources.Volumes...)
	}

	return nil
}

func getAutomountResources(api sync.ClusterAPI, namespace string) (*Resources, error) {
	gitCMAutoMountResources, err := ProvisionGitConfiguration(api, namespace)
	if err != nil {
		return nil, err
	}

	cmAutoMountResources, err := getDevWorkspaceConfigmaps(namespace, api)
	if err != nil {
		return nil, err
	}

	secretAutoMountResources, err := getDevWorkspaceSecrets(namespace, api)
	if err != nil {
		return nil, err
	}

	if gitCMAutoMountResources != nil && len(gitCMAutoMountResources.Volumes) > 0 {
		filterGitconfigAutomountVolume(cmAutoMountResources)
		filterGitconfigAutomountVolume(secretAutoMountResources)
	}

	cmAndSecretResources := mergeAutomountResources(cmAutoMountResources, secretAutoMountResources)
	mergedResources, err := mergeProjectedVolumes(cmAndSecretResources)
	if err != nil {
		return nil, err
	}
	dropItemsFieldFromVolumes(mergedResources.Volumes)

	pvcAutoMountResources, err := getAutoMountPVCs(namespace, api)
	if err != nil {
		return nil, err
	}

	return mergeAutomountResources(gitCMAutoMountResources, mergedResources, pvcAutoMountResources), nil
}

func checkAutomountVolumesForCollision(podAdditions *v1alpha1.PodAdditions, automount *Resources) error {
	// Get a map of automounted volume names to volume structs
	automountVolumeNames := map[string]corev1.Volume{}
	for _, volume := range automount.Volumes {
		automountVolumeNames[volume.Name] = volume
	}

	// Check that workspace volumes do not conflict with automounted volumes
	for _, volume := range podAdditions.Volumes {
		if conflict, exists := automountVolumeNames[volume.Name]; exists {
			return &dwerrors.FailError{
				Message: fmt.Sprintf("DevWorkspace volume '%s' conflicts with automounted volume from %s", volume.Name, formatVolumeDescription(conflict)),
			}
		}
	}

	// Check that automounted mountPaths do not collide
	automountVolumeMountsByMountPath := map[string]corev1.VolumeMount{}
	for _, vm := range automount.VolumeMounts {
		if conflict, exists := automountVolumeMountsByMountPath[vm.MountPath]; exists {
			return &dwerrors.FailError{
				Message: fmt.Sprintf("auto-mounted volumes from %s and %s have the same mount path",
					getVolumeDescriptionFromVolumeMount(vm, automount.Volumes), getVolumeDescriptionFromVolumeMount(conflict, automount.Volumes)),
			}
		}
		automountVolumeMountsByMountPath[vm.MountPath] = vm
	}

	// Check that automounted volume mountPaths do not conflict with existing mountPaths in any container
	for _, container := range podAdditions.Containers {
		for _, vm := range container.VolumeMounts {
			if conflict, exists := automountVolumeMountsByMountPath[vm.MountPath]; exists {
				return &dwerrors.FailError{
					Message: fmt.Sprintf("DevWorkspace volume %s in container %s has same mountpath as auto-mounted volume from %s",
						getVolumeDescriptionFromVolumeMount(vm, podAdditions.Volumes), container.Name, getVolumeDescriptionFromVolumeMount(conflict, automount.Volumes)),
				}
			}
		}
	}
	return nil
}

// getVolumeDescriptionFromAutoVolumeMount takes a volumeMount and list of volumes and returns a formatted description of
// the underlying volume (i.e. "secret <secretName>" or "configmap <configmapName>" as defined by formatVolumeDescription).
// If the volume referred to by the volumeMount does not exist in the volumes slice, the volumeMount's name is returned
func getVolumeDescriptionFromVolumeMount(vm corev1.VolumeMount, volumes []corev1.Volume) string {
	for _, volume := range volumes {
		if volume.Name == vm.Name {
			return formatVolumeDescription(volume)
		}
	}
	return vm.Name
}

// formatVolumeDescription formats a given volume as either "configmap '<configmap-name>'" or "secret '<secret-name>'",
// depending on whether the volume refers to a configmap or secret. If the volume is neither a secret nor configmap,
// returns the name of the volume itself.
func formatVolumeDescription(vol corev1.Volume) string {
	if vol.Secret != nil {
		return fmt.Sprintf("secret '%s'", vol.Secret.SecretName)
	} else if vol.ConfigMap != nil {
		return fmt.Sprintf("configmap '%s'", vol.ConfigMap.Name)
	} else if vol.PersistentVolumeClaim != nil {
		return fmt.Sprintf("pvc '%s'", vol.PersistentVolumeClaim.ClaimName)
	}
	return fmt.Sprintf("'%s'", vol.Name)
}

func mergeAutomountResources(resources ...*Resources) *Resources {
	result := &Resources{}
	for _, resource := range resources {
		if resource == nil {
			continue
		}
		result.Volumes = append(result.Volumes, resource.Volumes...)
		result.VolumeMounts = append(result.VolumeMounts, resource.VolumeMounts...)
		result.EnvFromSource = append(result.EnvFromSource, resource.EnvFromSource...)
	}
	return result
}

func flattenAutomountResources(resources []Resources) Resources {
	flattened := Resources{}
	for _, resource := range resources {
		flattened.Volumes = append(flattened.Volumes, resource.Volumes...)
		flattened.VolumeMounts = append(flattened.VolumeMounts, resource.VolumeMounts...)
		flattened.EnvFromSource = append(flattened.EnvFromSource, resource.EnvFromSource...)
	}
	return flattened
}

// findGitconfigAutomount searches a namespace for a automount resource (configmap or secret) that contains
// a system-wide gitconfig (i.e. the mountpath is `/etc/gitconfig`). Only objects with mount type "subpath"
// are considered. If a suitable object is found, the contents of the gitconfig defined there is returned.
func findGitconfigAutomount(api sync.ClusterAPI, namespace string) (gitconfig *string, err error) {
	configmapList := &corev1.ConfigMapList{}
	if err := api.Client.List(api.Ctx, configmapList, k8sclient.InNamespace(namespace), k8sclient.MatchingLabels{
		constants.DevWorkspaceMountLabel: "true",
	}); err != nil {
		return nil, err
	}
	for _, cm := range configmapList.Items {
		if cm.Annotations[constants.DevWorkspaceMountAsAnnotation] != constants.DevWorkspaceMountAsSubpath {
			continue
		}
		mountPath := cm.Annotations[constants.DevWorkspaceMountPathAnnotation]
		for key, value := range cm.Data {
			if path.Join(mountPath, key) == "/etc/gitconfig" {
				if gitconfig != nil {
					return nil, fmt.Errorf("duplicate automount keys on path /etc/gitconfig")
				}
				gitconfig = &value
			}
		}
	}

	secretList := &corev1.SecretList{}
	if err := api.Client.List(api.Ctx, secretList, k8sclient.InNamespace(namespace), k8sclient.MatchingLabels{
		constants.DevWorkspaceMountLabel: "true",
	}); err != nil {
		return nil, err
	}
	for _, secret := range secretList.Items {
		if secret.Annotations[constants.DevWorkspaceMountAsAnnotation] != constants.DevWorkspaceMountAsSubpath {
			continue
		}
		mountPath := secret.Annotations[constants.DevWorkspaceMountPathAnnotation]
		for key, value := range secret.Data {
			if path.Join(mountPath, key) == "/etc/gitconfig" {
				if gitconfig != nil {
					return nil, fmt.Errorf("duplicate automount keys on path /etc/gitconfig")
				}
				strValue := string(value)
				gitconfig = &strValue
			}
		}
	}
	return gitconfig, nil
}

func filterGitconfigAutomountVolume(resources *Resources) {
	var filteredVolumeMounts []corev1.VolumeMount
	var filteredVolumes []corev1.Volume
	var gitConfigVolumeName string
	volumeMountCounts := map[string]int{}
	for _, vm := range resources.VolumeMounts {
		if vm.MountPath == "/etc/gitconfig" {
			gitConfigVolumeName = vm.Name
			continue
		}
		filteredVolumeMounts = append(filteredVolumeMounts, vm)
		volumeMountCounts[vm.Name] += 1
	}
	removeGitconfigVolume := volumeMountCounts[gitConfigVolumeName] == 0
	for _, volume := range resources.Volumes {
		if volume.Name == gitConfigVolumeName && removeGitconfigVolume {
			continue
		}
		filteredVolumes = append(filteredVolumes, volume)
	}
	resources.VolumeMounts = filteredVolumeMounts
	resources.Volumes = filteredVolumes
}

// checkAutomountVolumeForPotentialError checks the configuration of an automount volume for potential errors
// that can be caught early and returned to more clearly warn the user. If no issues are found, returns empty string
func checkAutomountVolumeForPotentialError(obj k8sclient.Object) string {
	var objDesc string
	switch obj.(type) {
	case *corev1.Secret:
		objDesc = fmt.Sprintf("secret %s", obj.GetName())
	case *corev1.ConfigMap:
		objDesc = fmt.Sprintf("configmap %s", obj.GetName())
	}

	mountAs := obj.GetAnnotations()[constants.DevWorkspaceMountAsAnnotation]
	mountPath := obj.GetAnnotations()[constants.DevWorkspaceMountPathAnnotation]

	switch mountAs {
	case constants.DevWorkspaceMountAsEnv:
		if mountPath != "" {
			return fmt.Sprintf("automatically mounted %s should not define a mount path if it is mounted as environment variables", objDesc)
		}
	case constants.DevWorkspaceMountAsFile:
		if !strings.HasSuffix(mountPath, "/") {
			mountPath = mountPath + "/"
		}
		if strings.Contains(mountPath, ":") {
			return fmt.Sprintf("automatically mounted %s mount path cannot contain ':'", objDesc)
		}
		if mountPath == "/etc/" || mountPath == "/usr/" || mountPath == "/lib/" || mountPath == "/tmp/" {
			return fmt.Sprintf("automatically mounted %s is mounted as files but collides with system path %s -- mount as subpath instead", objDesc, mountPath)
		}
	}

	return ""
}

// getAccessModeForAutomount reads the access mode that should be used for an automounted configmap or secret
// by parsing the controller.devfile.io/mount-access-mode annotation. Returns an error if the access mode cannot
// be parse or is invalid (outside the range 0000-0777). If no annotation is present, a default value is returned.
func getAccessModeForAutomount(obj k8sclient.Object) (*int32, error) {
	accessModeStr := obj.GetAnnotations()[constants.DevWorkspaceMountAccessModeAnnotation]
	if accessModeStr == "" {
		return defaultAccessMode, nil
	}

	accessMode64, err := strconv.ParseInt(accessModeStr, 0, 32)
	if err != nil {
		return nil, err
	}
	if accessMode64 < 0 || accessMode64 > 0777 {
		return nil, fmt.Errorf("invalid access mode annotation: value '%s' parsed to %o (octal)", accessModeStr, accessMode64)
	}
	accessMode32 := int32(accessMode64)
	return &accessMode32, nil
}

// dropItemsFieldFromVolumes removes the items field from any secret or configmap pod volumes. This function is useful
// to avoid having to create a new pod if an item is added to the configmap or secret later, as the additional item
// will be propagated into the pod without changing the pod's spec.
func dropItemsFieldFromVolumes(volumes []corev1.Volume) {
	for idx, volume := range volumes {
		if volume.ConfigMap != nil {
			volumes[idx].ConfigMap.Items = nil
		} else if volume.Secret != nil {
			volumes[idx].Secret.Items = nil
		}
	}
}

func sortSecrets(secrets []corev1.Secret) {
	sort.Slice(secrets, func(i, j int) bool {
		return secrets[i].Name < secrets[j].Name
	})
}

func sortConfigmaps(cms []corev1.ConfigMap) {
	sort.Slice(cms, func(i, j int) bool {
		return cms[i].Name < cms[j].Name
	})
}
