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
	"strings"

	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
				"app.kubernetes.io/name":                  "async-storage", // TODO
				"app.kubernetes.io/part-of":               "devworkspace-operator",
				constants.DevWorkspaceWatchConfigMapLabel: "true",
			},
		},
		Data: map[string]string{
			authorizedKeysFilename: string(authorizedKeys),
		},
	}
	return cm
}

func getSSHAuthorizedKeysConfigMapCluster(namespace string, clusterAPI sync.ClusterAPI) (*corev1.ConfigMap, error) {
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

func removeAuthorizedKeyFromConfigMap(configmap *corev1.ConfigMap, authorizedKeyBytes []byte) (didChange bool, err error) {
	authorizedKeys, ok := configmap.Data[authorizedKeysFilename]
	if !ok {
		return false, fmt.Errorf("could not find authorized_keys in configmap %s", configmap.Name)
	}
	authorizedKey := string(authorizedKeyBytes)
	authorizedKeyTrimmed := strings.TrimRight(authorizedKey, "\n")
	changed := false
	var newKeys []string
	for _, key := range strings.Split(authorizedKeys, "\n") {
		if key == authorizedKeyTrimmed {
			changed = true
		} else {
			newKeys = append(newKeys, key)
		}
	}
	if changed {
		configmap.Data[authorizedKeysFilename] = strings.Join(newKeys, "\n")
	}
	return changed, nil
}
