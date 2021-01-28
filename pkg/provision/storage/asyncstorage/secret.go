//
// Copyright (c) 2019-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package asyncstorage

import (
	"fmt"

	devworkspace "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/controllers/workspace/provision"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	rsyncSSHKeyFilename = "rsync-via-ssh"
)

func GetSSHSidecarSecretName(workspaceId string) string {
	return fmt.Sprintf("%s-asyncsshkey", workspaceId)
}

func getSSHSidecarSecretSpec(workspace *devworkspace.DevWorkspace, privateKey []byte) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetSSHSidecarSecretName(workspace.Status.WorkspaceId),
			Namespace: workspace.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":    "async-storage", // TODO
				"app.kubernetes.io/part-of": "devworkspace-operator",
			},
		},
		Data: map[string][]byte{
			rsyncSSHKeyFilename: privateKey,
		},
		Type: corev1.SecretTypeOpaque,
	}

	return secret
}

func getSSHSidecarSecretCluster(workspace *devworkspace.DevWorkspace, clusterAPI provision.ClusterAPI) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	namespacedName := types.NamespacedName{
		Name:      GetSSHSidecarSecretName(workspace.Status.WorkspaceId),
		Namespace: workspace.Namespace,
	}
	err := clusterAPI.Client.Get(clusterAPI.Ctx, namespacedName, secret)
	return secret, err
}
