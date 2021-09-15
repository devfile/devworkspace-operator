//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package automount

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
func getDevWorkspaceGitConfig(client k8sclient.Client, namespace string) (*v1alpha1.PodAdditions, error) {
	secrets := &corev1.SecretList{}
	err := client.List(context.TODO(), secrets, k8sclient.InNamespace(namespace), k8sclient.MatchingLabels{
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
		configMapAdditions, err := mountGitConfigMap(gitCredentialsConfigMapName, mountpath, namespace, client)
		if err != nil {
			return podAdditions, err
		}
		podAdditions.Volumes = append(podAdditions.Volumes, configMapAdditions.Volumes...)
		podAdditions.VolumeMounts = append(podAdditions.VolumeMounts, configMapAdditions.VolumeMounts...)

		// mount the users git credentials
		joinedCredentials := strings.Join(credentials, "\n")
		secretAdditions, err := mountGitCredentialsSecret(gitCredentialsSecretName, mountpath, joinedCredentials, namespace, client)
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
func mountGitConfigMap(configMapName, mountPath, namespace string, client k8sclient.Client) (*v1alpha1.PodAdditions, error) {
	podAdditions := &v1alpha1.PodAdditions{}

	// Initialize the gitconfig template
	credentialsGitConfig := fmt.Sprintf(credentialTemplate, filepath.Join(mountPath, gitCredentialsName))

	// Create the configmap that stores the gitconfig
	err := createOrUpdateGitConfigMap(configMapName, namespace, credentialsGitConfig, client)
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
func mountGitCredentialsSecret(secretName, mountPath, credentials, namespace string, client k8sclient.Client) (*v1alpha1.PodAdditions, error) {
	podAdditions := &v1alpha1.PodAdditions{}

	// Create the configmap that stores all the users credentials
	err := createOrUpdateGitSecret(secretName, namespace, credentials, client)
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

func createOrUpdateGitSecret(secretName string, namespace string, config string, client k8sclient.Client) error {
	secret := getGitSecret(secretName, namespace, config)
	if err := client.Create(context.TODO(), secret); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		existingCfg, err := getClusterGitSecret(secretName, namespace, client)
		if err != nil {
			return err
		}
		secret.ResourceVersion = existingCfg.ResourceVersion
		err = client.Update(context.TODO(), secret)
		if err != nil {
			return err
		}
	}
	return nil
}

func getClusterGitSecret(secretName string, namespace string, client k8sclient.Client) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      secretName,
	}
	err := client.Get(context.TODO(), namespacedName, secret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return secret, nil
}

func getGitSecret(secretName string, namespace string, config string) *corev1.Secret {
	gitConfigMap := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":    "git-config-secret",
				"app.kubernetes.io/part-of": "devworkspace-operator",
			},
		},
		Data: map[string][]byte{
			gitCredentialsName: []byte(config),
		},
	}
	return gitConfigMap
}

func createOrUpdateGitConfigMap(configMapName string, namespace string, config string, client k8sclient.Client) error {
	configMap := getGitConfigMap(configMapName, namespace, config)
	if err := client.Create(context.TODO(), configMap); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		existingCfg, err := getClusterGitConfigMap(configMapName, namespace, client)
		if err != nil {
			return err
		}
		configMap.ResourceVersion = existingCfg.ResourceVersion
		err = client.Update(context.TODO(), configMap)
		if err != nil {
			return err
		}

	}
	return nil
}

func getClusterGitConfigMap(configMapName string, namespace string, client k8sclient.Client) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{}
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      configMapName,
	}
	err := client.Get(context.TODO(), namespacedName, configMap)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return configMap, nil
}

func getGitConfigMap(configMapName string, namespace string, config string) *corev1.ConfigMap {
	gitConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":    "git-config-secret",
				"app.kubernetes.io/part-of": "devworkspace-operator",
			},
		},
		Data: map[string]string{
			gitConfigName: config,
		},
	}

	return gitConfigMap
}
