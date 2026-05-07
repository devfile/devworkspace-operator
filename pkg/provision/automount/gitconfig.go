// Copyright (c) 2019-2026 Red Hat, Inc.
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
	"github.com/devfile/devworkspace-operator/pkg/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
)

const mergedGitCredentialsMountPath = "/.git-credentials/"

// ProvisionGitConfiguration takes care of mounting git credentials and a gitconfig into a devworkspace.
func ProvisionGitConfiguration(
	api sync.ClusterAPI,
	workspaceNamespace string,
	workspaceName string,
	workspaceDeployment *appsv1.Deployment,
) (*Resources, error) {
	credentialsSecrets, tlsConfigMaps, err := getGitResources(api, workspaceNamespace, workspaceName, workspaceDeployment)
	if err != nil {
		return nil, err
	}

	baseGitConfig, err := findGitconfigAutomount(api, workspaceNamespace, workspaceName)
	if err != nil {
		return nil, err
	}

	if len(credentialsSecrets) == 0 && len(tlsConfigMaps) == 0 && baseGitConfig == nil {
		// Remove any existing git configuration
		err := cleanupGitConfig(api, workspaceNamespace)
		return nil, err
	}

	mergedCredentialsSecret, err := mergeGitCredentials(workspaceNamespace, credentialsSecrets)
	if err != nil {
		return nil, &dwerrors.FailError{Message: "Failed to collect git credentials secrets", Err: err}
	}

	gitConfigMap, err := constructGitConfig(workspaceNamespace, mergedGitCredentialsMountPath, tlsConfigMaps, baseGitConfig)
	if err != nil {
		return nil, &dwerrors.FailError{Message: "Failed to prepare git config for workspace", Err: err}
	}

	if _, err = sync.SyncObjectWithCluster(mergedCredentialsSecret, api); err != nil {
		return nil, dwerrors.WrapSyncError(err)
	}

	if _, err = sync.SyncObjectWithCluster(gitConfigMap, api); err != nil {
		return nil, dwerrors.WrapSyncError(err)
	}

	resources := flattenAutomountResources([]Resources{
		getAutomountSecret(mergedGitCredentialsMountPath, constants.DevWorkspaceMountAsFile, defaultAccessMode, mergedCredentialsSecret),
		getAutomountConfigmap("/etc/", constants.DevWorkspaceMountAsSubpath, defaultAccessMode, gitConfigMap),
	})

	return &resources, nil
}

func getGitResources(
	api sync.ClusterAPI,
	workspaceNamespace string,
	workspaceName string,
	workspaceDeployment *appsv1.Deployment,
) (credentialSecrets []corev1.Secret, tlsConfigMaps []corev1.ConfigMap, err error) {
	credentialsLabelSelector := k8sclient.MatchingLabels{
		constants.DevWorkspaceGitCredentialLabel: "true",
	}
	tlsLabelSelector := k8sclient.MatchingLabels{
		constants.DevWorkspaceGitTLSLabel: "true",
	}

	secretList := &corev1.SecretList{}
	if err := api.Client.List(api.Ctx, secretList, k8sclient.InNamespace(workspaceNamespace), credentialsLabelSelector); err != nil {
		return nil, nil, err
	}

	// Filter resources by workspace name
	var secrets []corev1.Secret
	for _, secret := range secretList.Items {
		if !MatchesWorkspaceTarget(&secret, workspaceName) {
			log.V(1).Info("Skipping Git credentials Secret mount, workspace does not match include/exclude annotations", "namespace", secret.Namespace, "name", secret.Name, "workspace", workspaceName)
			continue
		}

		secrets = append(secrets, secret)
	}

	if len(secrets) > 0 {
		if canGitCredentialsMountWithoutRestart(secrets, workspaceDeployment) {
			sortSecrets(secrets)
		} else {
			// Cleanup slice, there are no Git credentials Secrets to mount
			secrets = nil
			log.V(1).Info("Skipping all Git credentials Secrets mount: resource requires workspace restart to be mounted", "namespace", workspaceNamespace, "workspace", workspaceName)
		}
	}

	configmapList := &corev1.ConfigMapList{}
	if err := api.Client.List(api.Ctx, configmapList, k8sclient.InNamespace(workspaceNamespace), tlsLabelSelector); err != nil {
		return nil, nil, err
	}

	// Filter resources by workspace name
	var configmaps []corev1.ConfigMap
	for _, cm := range configmapList.Items {
		if !MatchesWorkspaceTarget(&cm, workspaceName) {
			log.V(1).Info("Skipping Git ConfigMap mount, workspace does not match include/exclude annotations", "namespace", cm.Namespace, "name", cm.Name, "workspace", workspaceName)
			continue
		}

		configmaps = append(configmaps, cm)
	}

	// When git credentials are present, the gitconfig ConfigMap must be created
	// regardless of mount-on-start annotations.
	if len(configmaps) > 0 {
		if len(secrets) > 0 || canGitConfigsMountWithoutRestart(configmaps, workspaceDeployment) {
			sortConfigmaps(configmaps)
		} else {
			// Cleanup slice, there are no gitconfig to mount
			configmaps = nil
			log.V(1).Info("Skipping all Git ConfigMaps mount: resource requires workspace restart to be mounted", "namespace", workspaceNamespace, "workspace", workspaceName)
		}
	}

	return secrets, configmaps, nil
}

func cleanupGitConfig(api sync.ClusterAPI, namespace string) error {
	secretNN := types.NamespacedName{
		Name:      constants.GitCredentialsMergedSecretName,
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
		Name:      constants.GitCredentialsConfigMapName,
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

func canGitCredentialsMountWithoutRestart(secrets []corev1.Secret, workspaceDeployment *appsv1.Deployment) bool {
	volumeName := common.AutoMountSecretVolumeName(constants.GitCredentialsMergedSecretName)
	return canGitObjectsMountWithoutRestart(secrets, volumeName, workspaceDeployment)
}

func canGitConfigsMountWithoutRestart(configMaps []corev1.ConfigMap, workspaceDeployment *appsv1.Deployment) bool {
	volumeName := common.AutoMountConfigMapVolumeName(constants.GitCredentialsConfigMapName)
	return canGitObjectsMountWithoutRestart(configMaps, volumeName, workspaceDeployment)
}

func canGitObjectsMountWithoutRestart[T any](objs []T, volumeName string, workspaceDeployment *appsv1.Deployment) bool {
	// No deployment exists yet — workspace is not running, no restart risk
	if workspaceDeployment == nil {
		return true
	}

	// At least one object lacks mount-on-start
	if !allItemsMountOnStart(objs) {
		return true
	}

	automountResource := Resources{Volumes: []corev1.Volume{{Name: volumeName}}}

	// Volume is already mounted in the deployment, updating it won't cause a restart
	if isVolumeMountExistsInDeployment(automountResource, workspaceDeployment) {
		return true
	}

	return false
}

func allItemsMountOnStart[T any](objs []T) bool {
	for i := range objs {
		var obj interface{} = &objs[i]

		k8sObj, ok := obj.(k8sclient.Object)
		if !ok {
			continue
		}

		if !isMountOnStart(k8sObj) {
			return false
		}
	}

	return true
}
