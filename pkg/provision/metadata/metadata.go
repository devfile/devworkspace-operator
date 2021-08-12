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

package metadata

import (
	"context"
	"fmt"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	maputils "github.com/devfile/devworkspace-operator/internal/map"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	wsprovision "github.com/devfile/devworkspace-operator/pkg/provision/workspace"
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
func ProvisionWorkspaceMetadata(podAdditions *v1alpha1.PodAdditions, original, flattened *dw.DevWorkspace, api *wsprovision.ClusterAPI) error {
	cm, err := getSpecMetadataConfigMap(original, flattened)
	if err != nil {
		return err
	}
	err = controllerutil.SetControllerReference(original, cm, api.Scheme)
	if err != nil {
		return err
	}
	if inSync, err := syncConfigMapToCluster(cm, api); err != nil {
		return err
	} else if !inSync {
		return &NotReadyError{
			Message: "Waiting for DevWorkspace metadata configmap to be ready",
		}
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

func syncConfigMapToCluster(specCM *corev1.ConfigMap, api *wsprovision.ClusterAPI) (inSync bool, err error) {
	clusterCM := &corev1.ConfigMap{}
	err = api.Client.Get(context.TODO(), types.NamespacedName{Name: specCM.Name, Namespace: specCM.Namespace}, clusterCM)

	switch {
	case err == nil:
		if maputils.Equal(specCM.Data, clusterCM.Data) {
			return true, nil
		}
		clusterCM.Data = specCM.Data
		err = api.Client.Update(context.TODO(), clusterCM)
	case k8sErrors.IsNotFound(err):
		err = api.Client.Create(context.TODO(), specCM)
	default:
		return false, err
	}
	if k8sErrors.IsConflict(err) || k8sErrors.IsAlreadyExists(err) {
		return false, nil
	}
	return false, err
}

func getSpecMetadataConfigMap(original, flattened *dw.DevWorkspace) (*corev1.ConfigMap, error) {
	originalYaml, err := yaml.Marshal(original.Spec.Template)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal original DevWorkspace yaml: %w", err)
	}

	flattenedYaml, err := yaml.Marshal(flattened.Spec.Template)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal flattened DevWorkspace yaml: %w", err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.MetadataConfigMapName(original.Status.DevWorkspaceId),
			Namespace: original.Namespace,
			Labels:    constants.ControllerAppLabels(),
		},
		Data: map[string]string{
			originalYamlFilename:  string(originalYaml),
			flattenedYamlFilename: string(flattenedYaml),
		},
	}

	return cm, nil
}

func getVolumeFromConfigMap(cm *corev1.ConfigMap) *corev1.Volume {
	boolTrue := true
	defaultMode := int32(0644)
	return &corev1.Volume{
		Name: "workspace-metadata",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cm.Name,
				},
				Optional:    &boolTrue,
				DefaultMode: &defaultMode,
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
