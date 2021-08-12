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
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	wsprovision "github.com/devfile/devworkspace-operator/pkg/provision/workspace"
)

const (
	sshAuthorizedKeysConfigMapName = "async-storage-config"
	authorizedKeysFilename         = "authorized_keys"
)

func getSSHAuthorizedKeysConfigMapSpec(namespace string, authorizedKeys []byte) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sshAuthorizedKeysConfigMapName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":    "async-storage", // TODO
				"app.kubernetes.io/part-of": "devworkspace-operator",
			},
		},
		Data: map[string]string{
			authorizedKeysFilename: string(authorizedKeys),
		},
	}
	return cm
}

func getSSHAuthorizedKeysConfigMapCluster(namespace string, clusterAPI wsprovision.ClusterAPI) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{}
	namespaceName := types.NamespacedName{
		Name:      sshAuthorizedKeysConfigMapName,
		Namespace: namespace,
	}
	err := clusterAPI.Client.Get(clusterAPI.Ctx, namespaceName, cm)
	return cm, err
}

func addAuthorizedKeyToConfigMap(configmap *corev1.ConfigMap, authorizedKeyBytes []byte) (didChange bool, err error) {
	authorizedKeys, ok := configmap.Data[authorizedKeysFilename]
	if !ok {
		return false, fmt.Errorf("could not find authorized_keys in configmap %s", configmap.Name)
	}
	authorizedKey := string(authorizedKeyBytes)
	authorizedKeyTrimmed := strings.TrimRight(authorizedKey, "\n")
	exists := false
	for _, key := range strings.Split(authorizedKeys, "\n") {
		if key == authorizedKeyTrimmed {
			exists = true
			break
		}
	}
	if exists {
		return false, nil
	}
	configmap.Data[authorizedKeysFilename] = authorizedKeys + authorizedKey
	return true, nil
}
