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
	"path/filepath"
	"strings"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const gitCredentialsName = "credentials"
const gitCredentialsSecretName = "devworkspace-merged-git-credentials"

// provisionUserGitCredentials takes care of mounting git user credentials into a devworkspace.
//	It does so by:
//		1. Finding all secrets labeled with "controller.devfile.io/git-credential": "true" and grabbing all the user credentials
//			and condensing them into one string
//		2. Creating and mounting a secret named gitCredentialsSecretName into the workspace pod
func provisionUserGitCredentials(client k8sclient.Client, namespace string, mountpath string, credentials []string) (*v1alpha1.PodAdditions, error) {
	// mount the users git credentials
	joinedCredentials := strings.Join(credentials, "\n")
	secretAdditions, err := mountGitCredentialsSecret(mountpath, joinedCredentials, namespace, client)
	if err != nil {
		return nil, err
	}
	return secretAdditions, nil
}

// mountGitCredentialsSecret mounts the users git credentials to mountpath/credentials
//   It does so by:
//		1. Creating the secret that stores the credentials if it does not exist
//		2. Adding the new secret volume and volume mount to the pod additions
func mountGitCredentialsSecret(mountPath, credentials, namespace string, client k8sclient.Client) (*v1alpha1.PodAdditions, error) {
	podAdditions := &v1alpha1.PodAdditions{}

	// Create the configmap that stores all the users credentials
	err := createOrUpdateGitSecret(gitCredentialsSecretName, namespace, credentials, client)
	if err != nil {
		return nil, err
	}

	// Create the volume for the secret
	podAdditions.Volumes = append(podAdditions.Volumes, GetAutoMountVolumeWithSecret(gitCredentialsSecretName))

	// Create the git credentials volume mount and set it's location as mountpath/credentials
	gitSecretVolumeMount := getGitCredentialsVolumeMount(mountPath, gitCredentialsSecretName)
	podAdditions.VolumeMounts = append(podAdditions.VolumeMounts, gitSecretVolumeMount)

	return podAdditions, nil
}

func getGitCredentialsVolumeMount(mountPath string, secretName string) corev1.VolumeMount {
	gitSecretVolumeMount := GetAutoMountSecretVolumeMount(filepath.Join(mountPath, gitCredentialsName), secretName)
	gitSecretVolumeMount.ReadOnly = true
	gitSecretVolumeMount.SubPath = gitCredentialsName
	return gitSecretVolumeMount
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
				"app.kubernetes.io/defaultName": "git-config-secret",
				"app.kubernetes.io/part-of":     "devworkspace-operator",
			},
		},
		Data: map[string][]byte{
			gitCredentialsName: []byte(config),
		},
	}
	return gitConfigMap
}
