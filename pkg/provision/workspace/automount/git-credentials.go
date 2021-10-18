//
// Copyright (c) 2019-2021 Red Hat, Inc.
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

package automount

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const gitCredentialsName = "credentials"
const gitConfigName = "gitconfig"
const gitConfigLocation = "/etc/" + gitConfigName
const gitCredentialsSecretName = "devworkspace-merged-git-credentials"
const gitCredentialsConfigMapName = "devworkspace-gitconfig"
const credentialTemplate = "[credential]\n\thelper = store --file %s\n"

// getDevWorkspaceGitConfig takes care of mounting git credentials and a gitconfig into a devworkspace.
//	It does so by:
//		1. Finding all secrets labeled with "controller.devfile.io/git-credential": "true" and grabbing all their credentials
//			and condensing them into one string
//		2. Creating and mounting a gitconfig config map to /etc/gitconfig that points to where the credentials are stored
//		3. Creating and mounting a credentials secret to mountpath/credentials that stores the users git credentials
func getDevWorkspaceGitConfig(api sync.ClusterAPI, namespace string) (*v1alpha1.PodAdditions, error) {
	secrets := &corev1.SecretList{}
	err := api.Client.List(api.Ctx, secrets, k8sclient.InNamespace(namespace), k8sclient.MatchingLabels{
		constants.DevWorkspaceGitCredentialLabel: "true",
	})
	if err != nil {
		return nil, err
	}
	var credentials []string
	var mountpath string
	for _, secret := range secrets.Items {
		credentials = append(credentials, string(secret.Data[gitCredentialsName]))
		if val, ok := secret.Annotations[constants.DevWorkspaceMountPathAnnotation]; ok {
			if mountpath != "" && val != mountpath {
				return nil, &FatalError{fmt.Errorf("auto-mounted git credentials have conflicting mountPaths")}
			}
			mountpath = val
		}
	}

	podAdditions := &v1alpha1.PodAdditions{}
	if len(credentials) > 0 {
		// mount the gitconfig
		configMapAdditions, err := mountGitConfigMap(gitCredentialsConfigMapName, mountpath, namespace, api)
		if err != nil {
			return podAdditions, err
		}
		podAdditions.Volumes = append(podAdditions.Volumes, configMapAdditions.Volumes...)
		podAdditions.VolumeMounts = append(podAdditions.VolumeMounts, configMapAdditions.VolumeMounts...)

		// mount the users git credentials
		joinedCredentials := strings.Join(credentials, "\n")
		secretAdditions, err := mountGitCredentialsSecret(gitCredentialsSecretName, mountpath, joinedCredentials, namespace, api)
		if err != nil {
			return podAdditions, err
		}
		podAdditions.Volumes = append(podAdditions.Volumes, secretAdditions.Volumes...)
		podAdditions.VolumeMounts = append(podAdditions.VolumeMounts, secretAdditions.VolumeMounts...)
	}
	return podAdditions, nil
}

// mountGitConfigMap mounts the gitconfig to /etc/gitconfig in all devworkspaces in the given namespace.
//   It does so by:
//		1. Creating the configmap that stores the gitconfig if it does not exist
//		2. Setting the proper owner ref to the devworkspace
//		3. Adding the new config map volume and volume mount to the pod additions
func mountGitConfigMap(configMapName, mountPath, namespace string, api sync.ClusterAPI) (*v1alpha1.PodAdditions, error) {
	podAdditions := &v1alpha1.PodAdditions{}

	// Initialize the gitconfig template
	credentialsGitConfig := fmt.Sprintf(credentialTemplate, filepath.Join(mountPath, gitCredentialsName))

	// Create the configmap that stores the gitconfig
	err := createOrUpdateGitConfigMap(configMapName, namespace, credentialsGitConfig, api)
	if err != nil {
		return nil, err
	}

	// Create the volume for the configmap
	podAdditions.Volumes = append(podAdditions.Volumes, GetAutoMountVolumeWithConfigMap(configMapName))

	// Create the gitconfig volume mount and set it's location as /etc/gitconfig so it's automatically picked up by git
	gitConfigMapVolumeMount := GetAutoMountConfigMapVolumeMount(gitConfigLocation, configMapName)
	gitConfigMapVolumeMount.SubPath = gitConfigName
	gitConfigMapVolumeMount.ReadOnly = false
	podAdditions.VolumeMounts = append(podAdditions.VolumeMounts, gitConfigMapVolumeMount)

	return podAdditions, nil
}

// mountGitCredentialsSecret mounts the users git credentials to mountpath/credentials
//   It does so by:
//		1. Creating the secret that stores the credentials if it does not exist
//		2. Setting the proper owner ref to the devworkspace
//		3. Adding the new secret volume and volume mount to the pod additions
func mountGitCredentialsSecret(secretName, mountPath, credentials, namespace string, api sync.ClusterAPI) (*v1alpha1.PodAdditions, error) {
	podAdditions := &v1alpha1.PodAdditions{}

	// Create the configmap that stores all the users credentials
	err := createOrUpdateGitSecret(secretName, namespace, credentials, api)
	if err != nil {
		return nil, err
	}

	// Create the volume for the secret
	podAdditions.Volumes = append(podAdditions.Volumes, GetAutoMountVolumeWithSecret(secretName))

	// Create the git credentials volume mount and set it's location as mountpath/credentials
	gitSecretVolumeMount := GetAutoMountSecretVolumeMount(filepath.Join(mountPath, gitCredentialsName), secretName)
	gitSecretVolumeMount.ReadOnly = true
	gitSecretVolumeMount.SubPath = gitCredentialsName
	podAdditions.VolumeMounts = append(podAdditions.VolumeMounts, gitSecretVolumeMount)

	return podAdditions, nil
}

func createOrUpdateGitSecret(secretName string, namespace string, config string, api sync.ClusterAPI) error {
	secret := getGitSecret(secretName, namespace, config)
	_, err := sync.SyncObjectWithCluster(secret, api)
	switch t := err.(type) {
	case nil:
		return nil
	case *sync.NotInSyncError:
		return nil // Continue optimistically (as originally implemented)
	case *sync.UnrecoverableSyncError:
		return &FatalError{Err: t.Cause}
	default:
		return err
	}
}

func getGitSecret(secretName string, namespace string, config string) *corev1.Secret {
	gitConfigMap := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":               "git-config-secret",
				"app.kubernetes.io/part-of":            "devworkspace-operator",
				constants.DevWorkspaceWatchSecretLabel: "true",
			},
		},
		Data: map[string][]byte{
			gitCredentialsName: []byte(config),
		},
	}
	return gitConfigMap
}

func createOrUpdateGitConfigMap(configMapName string, namespace string, config string, api sync.ClusterAPI) error {
	configMap := getGitConfigMap(configMapName, namespace, config)
	_, err := sync.SyncObjectWithCluster(configMap, api)
	switch t := err.(type) {
	case nil:
		return nil
	case *sync.NotInSyncError:
		return nil // Continue optimistically (as originally implemented)
	case *sync.UnrecoverableSyncError:
		return &FatalError{Err: t.Cause}
	default:
		return err
	}
}

func getGitConfigMap(configMapName string, namespace string, config string) *corev1.ConfigMap {
	gitConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":                  "git-config-secret",
				"app.kubernetes.io/part-of":               "devworkspace-operator",
				constants.DevWorkspaceWatchConfigMapLabel: "true",
			},
		},
		Data: map[string]string{
			gitConfigName: config,
		},
	}

	return gitConfigMap
}
