// Copyright (c) 2019-2022 Red Hat, Inc.
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

package automount

import (
	"fmt"

	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// ProvisionGitConfiguration takes care of mounting git credentials and a gitconfig into a devworkspace.
func ProvisionGitConfiguration(api sync.ClusterAPI, namespace string) (*Resources, error) {
	credentialsSecrets, tlsConfigMaps, err := getGitResources(api, namespace)
	if err != nil {
		return nil, err
	}
	if len(credentialsSecrets) == 0 && len(tlsConfigMaps) == 0 {
		// Remove any existing git configuration
		err := cleanupGitConfig(api, namespace)
		return nil, err
	}
	baseGitConfig, err := findGitconfigAutomount(api, namespace)
	if err != nil {
		return nil, err
	}

	mergedCredentialsSecret, err := mergeGitCredentials(namespace, credentialsSecrets)
	if err != nil {
		return nil, &AutoMountError{IsFatal: true, Err: err}
	}

	credentialsMountPath, err := getCredentialsMountPath(credentialsSecrets)
	if err != nil {
		return nil, &AutoMountError{IsFatal: true, Err: err}
	}

	gitConfigMap, err := constructGitConfig(namespace, credentialsMountPath, tlsConfigMaps, baseGitConfig)
	if err != nil {
		return nil, &AutoMountError{IsFatal: true, Err: err}
	}

	_, err = sync.SyncObjectWithCluster(mergedCredentialsSecret, api)
	switch t := err.(type) {
	case nil:
		break
	case *sync.NotInSyncError:
		return nil, &AutoMountError{IsFatal: false, Err: fmt.Errorf("syncing merged git credentials secret: %w", t)}
	case *sync.UnrecoverableSyncError:
		return nil, &AutoMountError{IsFatal: true, Err: fmt.Errorf("failed to sync merged git credentials secret: %w", t.Cause)}
	default:
		return nil, &AutoMountError{IsFatal: false, Err: err}
	}

	_, err = sync.SyncObjectWithCluster(gitConfigMap, api)
	switch t := err.(type) {
	case nil:
		break
	case *sync.NotInSyncError:
		return nil, &AutoMountError{IsFatal: false, Err: fmt.Errorf("syncing gitconfig configmap: %w", t)}
	case *sync.UnrecoverableSyncError:
		return nil, &AutoMountError{IsFatal: true, Err: fmt.Errorf("failed to sync gitconfig configmap: %w", t.Cause)}
	default:
		return nil, &AutoMountError{IsFatal: false, Err: err}
	}
	resources := flattenAutomountResources([]Resources{
		getAutomountSecret(credentialsMountPath, constants.DevWorkspaceMountAsSubpath, mergedCredentialsSecret),
		getAutomountConfigmap("/etc/", constants.DevWorkspaceMountAsSubpath, gitConfigMap),
	})

	return &resources, nil
}

func getGitResources(api sync.ClusterAPI, namespace string) (credentialSecrets []corev1.Secret, tlsConfigMaps []corev1.ConfigMap, err error) {
	credentialsLabelSelector := k8sclient.MatchingLabels{
		constants.DevWorkspaceGitCredentialLabel: "true",
	}
	tlsLabelSelector := k8sclient.MatchingLabels{
		constants.DevWorkspaceGitTLSLabel: "true",
	}

	secretList := &corev1.SecretList{}
	if err := api.Client.List(api.Ctx, secretList, k8sclient.InNamespace(namespace), credentialsLabelSelector); err != nil {
		return nil, nil, err
	}
	var secrets []corev1.Secret
	if len(secretList.Items) > 0 {
		secrets = secretList.Items
	}

	configmapList := &corev1.ConfigMapList{}
	if err := api.Client.List(api.Ctx, configmapList, k8sclient.InNamespace(namespace), tlsLabelSelector); err != nil {
		return nil, nil, err
	}
	var configmaps []corev1.ConfigMap
	if len(configmapList.Items) > 0 {
		configmaps = configmapList.Items
	}

	return secrets, configmaps, nil
}

func cleanupGitConfig(api sync.ClusterAPI, namespace string) error {
	secretNN := types.NamespacedName{
		Name:      gitCredentialsSecretName,
		Namespace: namespace,
	}
	tlsSecret := &corev1.Secret{}
	err := api.Client.Get(api.Ctx, secretNN, tlsSecret)
	switch {
	case err == nil:
		err := api.Client.Delete(api.Ctx, tlsSecret)
		if err != nil && !k8sErrors.IsNotFound(err) {
			return err
		}
	case k8sErrors.IsNotFound(err):
		break
	default:
		return err
	}

	configmapNN := types.NamespacedName{
		Name:      gitCredentialsConfigMapName,
		Namespace: namespace,
	}
	credentialsConfigMap := &corev1.ConfigMap{}
	err = api.Client.Get(api.Ctx, configmapNN, credentialsConfigMap)
	switch {
	case err == nil:
		err := api.Client.Delete(api.Ctx, credentialsConfigMap)
		if err != nil && !k8sErrors.IsNotFound(err) {
			return err
		}
	case k8sErrors.IsNotFound(err):
		break
	default:
		return err
	}

	return nil
}
