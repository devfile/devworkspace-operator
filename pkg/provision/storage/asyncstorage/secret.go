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

package asyncstorage

import (
	"fmt"

	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/devfile/devworkspace-operator/pkg/constants"
)

const (
	rsyncSSHKeyFilename = "rsync-via-ssh"
)

func GetSSHSidecarSecretName(workspaceId string) string {
	return fmt.Sprintf("%s-asyncsshkey", workspaceId)
}

func getSSHSidecarSecretSpec(workspace *common.DevWorkspaceWithConfig, privateKey []byte) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetSSHSidecarSecretName(workspace.Status.DevWorkspaceId),
			Namespace: workspace.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":               "async-storage", // TODO
				"app.kubernetes.io/part-of":            "devworkspace-operator",
				constants.DevWorkspaceWatchSecretLabel: "true",
			},
			Finalizers: []string{
				asyncStorageFinalizer,
			},
		},
		Data: map[string][]byte{
			rsyncSSHKeyFilename: privateKey,
		},
		Type: corev1.SecretTypeOpaque,
	}
	return secret
}

func getSSHSidecarSecretCluster(workspace *common.DevWorkspaceWithConfig, clusterAPI sync.ClusterAPI) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	namespacedName := types.NamespacedName{
		Name:      GetSSHSidecarSecretName(workspace.Status.DevWorkspaceId),
		Namespace: workspace.Namespace,
	}
	err := clusterAPI.Client.Get(clusterAPI.Ctx, namespacedName, secret)
	return secret, err
}
