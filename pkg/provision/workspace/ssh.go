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
	_ "embed"
	"path"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

const SshAskPassMountPath = "/.ssh-askpass/"
const SshAskPassScriptFileName = "ssh-askpass.sh"

//go:embed ssh-askpass.sh
var data string

func ProvisionSshAskPass(api sync.ClusterAPI, namespace string, podAdditions *v1alpha1.PodAdditions) error {
	sshAskPassConfigMap := constructSshAskPassCM(namespace)
	if _, err := sync.SyncObjectWithCluster(sshAskPassConfigMap, api); err != nil {
		switch err.(type) {
		case *sync.NotInSyncError: // Ignore the object created error
		default:
			return dwerrors.WrapSyncError(err)
		}
	}

	sshAskPassVolumeMounts, sshAskPassVolumes, err := getSshAskPassVolumesAndVolumeMounts()
	if err != nil {
		return err
	}
	podAdditions.VolumeMounts = append(podAdditions.VolumeMounts, sshAskPassVolumeMounts...)
	podAdditions.Volumes = append(podAdditions.Volumes, sshAskPassVolumes...)
	return nil
}

func constructSshAskPassCM(namespace string) *corev1.ConfigMap {
	askPassConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.SshAskPassConfigMapName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/defaultName":         "ssh-askpass-secret",
				"app.kubernetes.io/part-of":             "devworkspace-operator",
				"controller.devfile.io/watch-configmap": "true",
			},
		},
		Data: map[string]string{
			SshAskPassScriptFileName: data,
		},
	}
	return askPassConfigMap
}

func getSshAskPassVolumesAndVolumeMounts() ([]corev1.VolumeMount, []corev1.Volume, error) {
	name := "ssh-askpass"
	volume := corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: constants.SshAskPassConfigMapName,
				},
				DefaultMode: pointer.Int32(0755),
			},
		},
	}
	volumeMount := corev1.VolumeMount{
		Name:      name,
		ReadOnly:  true,
		MountPath: path.Join(SshAskPassMountPath, SshAskPassScriptFileName),
		SubPath:   SshAskPassScriptFileName,
	}
	return []corev1.VolumeMount{volumeMount}, []corev1.Volume{volume}, nil
}
