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

package metadata

import (
	"fmt"

	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

const (
	// originalYamlFilename is the filename mounted to workspace containers which contains the current DevWorkspace yaml
	originalYamlFilename = "original.devworkspace.yaml"

	// flattenedYamlFilename is the filename mounted to workspace containers which contains the flattened (i.e.
	// resolved plugins and parent) DevWorkspace yaml
	flattenedYamlFilename = "flattened.devworkspace.yaml"

	// metadataMountPath is where files containing workspace metadata are mounted
	metadataMountPath = "/devworkspace-metadata"
)

// ProvisionWorkspaceMetadata creates a configmap on the cluster that stores metadata about the workspace and configures all
// workspace containers to mount that configmap at /devworkspace-metadata. Each container has the environment
// variable DEVWORKSPACE_METADATA set to the mount path for the configmap
func ProvisionWorkspaceMetadata(podAdditions *v1alpha1.PodAdditions, original, flattened *common.DevWorkspaceWithConfig, api sync.ClusterAPI) error {
	cm, err := getSpecMetadataConfigMap(original, flattened)
	if err != nil {
		return err
	}
	err = controllerutil.SetControllerReference(original.DevWorkspace, cm, api.Scheme)
	if err != nil {
		return err
	}
	if _, err = sync.SyncObjectWithCluster(cm, api); err != nil {
		dwerrors.WrapSyncError(err)
	}

	vol := getVolumeFromConfigMap(cm)
	podAdditions.Volumes = append(podAdditions.Volumes, *vol)
	vm := getVolumeMountFromVolume(vol)
	podAdditions.VolumeMounts = append(podAdditions.VolumeMounts, *vm)

	for idx := range podAdditions.Containers {
		podAdditions.Containers[idx].Env = append(podAdditions.Containers[idx].Env, getWorkspaceMetaEnvVar()...)
	}

	for idx := range podAdditions.InitContainers {
		podAdditions.InitContainers[idx].Env = append(podAdditions.InitContainers[idx].Env, getWorkspaceMetaEnvVar()...)
	}

	return nil
}

func getSpecMetadataConfigMap(original, flattened *common.DevWorkspaceWithConfig) (*corev1.ConfigMap, error) {
	originalYaml, err := yaml.Marshal(original.Spec.Template)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal original DevWorkspace yaml: %w", err)
	}

	flattenedYaml, err := yaml.Marshal(flattened.Spec.Template)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal flattened DevWorkspace yaml: %w", err)
	}

	cmLabels := constants.ControllerAppLabels()
	cmLabels[constants.DevWorkspaceWatchConfigMapLabel] = "true"
	cmLabels[constants.DevWorkspaceIDLabel] = original.Status.DevWorkspaceId
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.MetadataConfigMapName(original.Status.DevWorkspaceId),
			Namespace: original.Namespace,
			Labels:    cmLabels,
		},
		Data: map[string]string{
			originalYamlFilename:  string(originalYaml),
			flattenedYamlFilename: string(flattenedYaml),
		},
	}

	return cm, nil
}

func getVolumeFromConfigMap(cm *corev1.ConfigMap) *corev1.Volume {
	return &corev1.Volume{
		Name: "workspace-metadata",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cm.Name,
				},
				Optional:    pointer.Bool(true),
				DefaultMode: pointer.Int32(0644),
			},
		},
	}
}

func getVolumeMountFromVolume(vol *corev1.Volume) *corev1.VolumeMount {
	return &corev1.VolumeMount{
		Name:      vol.Name,
		ReadOnly:  true,
		MountPath: metadataMountPath,
	}
}
