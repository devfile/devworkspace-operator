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

package provision

import (
	"context"
	"crypto/rand"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/che-incubator/che-workspace-operator/pkg/common"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateAndSyncSecret creates a secret with name secretName and syncs it with the cluster
// If secretName is already present on the cluster the secret value will not be updated
func CreateAndSyncSecret(secretName, workspaceID string, client client.Client, reqLogger logr.Logger) (err error) {
	var secret *corev1.Secret
	if secret, err = generateRandomizedSecret(secretName, workspaceID); err != nil {
		return err
	}

	clusterSecret, err := getClusterSecret(client, workspaceID)
	if errors.IsNotFound(err) {
		if err := SyncObject(secret, client, reqLogger); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	mergedMap := mergeMapKeys(clusterSecret.Data, secret.Data)
	secret.Data = mergedMap

	if err := SyncMutableObject(secret, client, reqLogger); err != nil {
		return err
	}
	return nil
}

// Merge all keys that are present in newMap into oldMap
func mergeMapKeys(oldMap map[string][]byte, newMap map[string][]byte) map[string][]byte {
	for k, v := range newMap {
		if _, ok := oldMap[k]; !ok {
			oldMap[k] = v
		}
	}
	return oldMap
}

func getClusterSecret(client client.Client, workspaceID string) (*corev1.Secret, error) {
	found := &corev1.Secret{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: common.SecretStore(workspaceID), Namespace: "che-workspace-controller"}, found)
	if err != nil {
		return nil, err
	}
	return found, nil
}

func generateRandomizedSecret(secretName, workspaceID string) (*corev1.Secret, error) {
	randomCookieSecret, err := generateRandomBase(16)
	if err != nil {
		return nil, err
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.SecretStore(workspaceID),
			Namespace: "che-workspace-controller",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		Data: map[string][]byte{
			secretName: []byte(randomCookieSecret),
		},
	}, nil
}

func generateRandomBase(base int) (string, error) {
	bytes := make([]byte, base)

	_, err := rand.Read(bytes[:])
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%X", bytes), nil
}
