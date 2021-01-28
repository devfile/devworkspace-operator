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
	devworkspace "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/controllers/workspace/provision"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// GetOrCreateSSHConfig returns the secret and configmap used for the asynchronous deployment. The Secret is generated per-workspace
// and should be mounted to the asynchronous storage sync sidecar. The ConfigMap is per-namespace and stores authorized_keys for each
// workspace that is expected to use asynchronous storage; it should be mounted in the asynchronous storage sync deployment.
//
// If the k8s objects do not exist, an SSH keypair is generated and a secret and configmap are created on the cluster.
// This function works on two streams:
// 1. If the async storage SSH secret for the given workspace does not exist on the cluster, an SSH keypair are generated, a
//    Secret is synced to the cluster and the corresponding authorized key is added to the ConfigMap
// 2. If the async storage SSH secret exists, its content is read, and the ConfigMap is verified to contain the corresponding public
//    key in authorized_keys.
// In both cases, if the ConfigMap does not exist, it is created.
//
// Returns NotReadyError if changes were made to the cluster.
func GetOrCreateSSHConfig(workspace *devworkspace.DevWorkspace, clusterAPI provision.ClusterAPI) (*corev1.Secret, *corev1.ConfigMap, error) {
	var pubKey []byte
	clusterSecret, err := getSSHSidecarSecretCluster(workspace, clusterAPI)
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			return nil, nil, err
		}
		// Secret does not exist; generate new SSH keypair and create secret
		var privateKey []byte
		pubKey, privateKey, err = GetSSHKeyPair()
		if err != nil {
			return nil, nil, err
		}
		specSecret := getSSHSidecarSecretSpec(workspace, privateKey)
		err := controllerutil.SetControllerReference(workspace, specSecret, clusterAPI.Scheme)
		if err != nil {
			return nil, nil, err
		}
		// Create secret now to make sure we don't add pubKey to the configmap and then fail to create corresponding secret
		err = clusterAPI.Client.Create(clusterAPI.Ctx, specSecret)
		if err != nil && !k8sErrors.IsAlreadyExists(err) {
			return nil, nil, err
		}
		return nil, nil, NotReadyError
	} else {
		// Secret exists; extract SSH keypair from it
		pubKey, _, err = ExtractSSHKeyPairFromSecret(clusterSecret)
		if err != nil {
			return nil, nil, err
		}
	}

	clusterConfigMap, err := getSSHAuthorizedKeysConfigMapCluster(workspace.Namespace, clusterAPI)
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			return nil, nil, err
		}
		// ConfigMap does not yet exist; create ConfigMap with pubKey from secret
		specCM := getSSHAuthorizedKeysConfigMapSpec(workspace.Namespace, pubKey)
		err := clusterAPI.Client.Create(clusterAPI.Ctx, specCM)
		if err != nil && !k8sErrors.IsAlreadyExists(err) {
			return nil, nil, err
		}
		return nil, nil, NotReadyError
	} else {
		// ConfigMap exists; verify that current pubkey is in authorized_keys and add it if necessary
		didChange, err := addAuthorizedKeyToConfigMap(clusterConfigMap, pubKey)
		if err != nil {
			return nil, nil, err
		}
		if didChange {
			err := clusterAPI.Client.Update(clusterAPI.Ctx, clusterConfigMap)
			if err != nil && !k8sErrors.IsConflict(err) {
				return nil, nil, err
			}
			return nil, nil, NotReadyError
		}
	}
	return clusterSecret, clusterConfigMap, nil
}
