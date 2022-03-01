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
	"path"
	"strings"

	"github.com/devfile/devworkspace-operator/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const gitTLSHostKey = "host"
const gitTLSCertificateKey = "certificate"

const gitConfigName = "gitconfig"
const gitConfigLocation = "/etc/" + gitConfigName
const gitCredentialsConfigMapName = "devworkspace-gitconfig"

const gitCredentialsSecretKey = "credentials"
const gitCredentialsSecretName = "devworkspace-merged-git-credentials"

const credentialTemplate = `[credential]
    helper = store --file %s
`

const gitServerTemplate = `[http "%s"]
    sslCAInfo = %s
`

const defaultGitServerTemplate = `[http]
    sslCAInfo = %s
`

func constructGitConfig(namespace, credentialMountPath string, certificatesConfigMaps []corev1.ConfigMap, baseGitConfig *string) (*corev1.ConfigMap, error) {
	var configSettings []string
	if credentialMountPath != "" {
		configSettings = append(configSettings, fmt.Sprintf(credentialTemplate, path.Join(credentialMountPath, gitCredentialsSecretKey)))
	}

	if baseGitConfig != nil {
		configSettings = append(configSettings, *baseGitConfig)
	}

	defaultTLSFound := false
	for _, cm := range certificatesConfigMaps {
		if _, certFound := cm.Data[gitTLSCertificateKey]; !certFound {
			return nil, fmt.Errorf("could not find certificate field in configmap %s", cm.Name)
		}

		mountPath := cm.Annotations[constants.DevWorkspaceMountPathAnnotation]
		if mountPath == "" {
			mountPath = fmt.Sprintf("/etc/config/%s", cm.Name)
		}
		certificatePath := path.Join(mountPath, gitTLSCertificateKey)

		host, hostFound := cm.Data[gitTLSHostKey]
		if !hostFound {
			if defaultTLSFound {
				return nil, fmt.Errorf("multiple git tls credentials do not have host specified")
			}
			configSettings = append(configSettings, fmt.Sprintf(defaultGitServerTemplate, certificatePath))
			defaultTLSFound = true
		} else {
			configSettings = append(configSettings, fmt.Sprintf(gitServerTemplate, host, certificatePath))
		}
	}

	gitConfig := strings.Join(configSettings, "\n")

	gitConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gitCredentialsConfigMapName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/defaultName":         "git-config-secret",
				"app.kubernetes.io/part-of":             "devworkspace-operator",
				"controller.devfile.io/watch-configmap": "true",
			},
		},
		Data: map[string]string{
			gitConfigName: gitConfig,
		},
	}

	return gitConfigMap, nil
}

func mergeGitCredentials(namespace string, credentialSecrets []corev1.Secret) (*corev1.Secret, error) {
	var allCredentials []string
	for _, credentialSecret := range credentialSecrets {
		credential, found := credentialSecret.Data[gitCredentialsSecretKey]
		if !found {
			return nil, fmt.Errorf("git-credentials secret %s does not contain data in key %s", credentialSecret.Name, gitCredentialsSecretKey)
		}
		allCredentials = append(allCredentials, string(credential))
	}
	mergedCredentials := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gitCredentialsSecretName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/defaultName":      "git-config-secret",
				"app.kubernetes.io/part-of":          "devworkspace-operator",
				"controller.devfile.io/watch-secret": "true",
			},
		},
		Data: map[string][]byte{
			gitCredentialsSecretKey: []byte(strings.Join(allCredentials, "\n")),
		},
		Type: corev1.SecretTypeOpaque,
	}
	return mergedCredentials, nil
}

// getCredentialsMountPath returns the mount path to be used by all git credentials secrets. If no secrets define a mountPath,
// the root path ('/credentials') is used. If secrets define conflicting mountPaths, an error is returned and represents an invalid
// configuration. If any secret defines a mountPath, that mountPath overrides the mountPath for all secrets that do not
// define a mountPath. If there are no credentials secrets, the empty string is returned
func getCredentialsMountPath(secrets []corev1.Secret) (string, error) {
	if len(secrets) == 0 {
		return "", nil
	}
	mountPath := ""
	for _, secret := range secrets {
		secretMountPath := secret.Annotations[constants.DevWorkspaceMountPathAnnotation]
		if secretMountPath != "" {
			if mountPath != "" && secretMountPath != mountPath {
				return "", fmt.Errorf("auto-mounted git credentials have conflicting mountPaths: %s, %s", mountPath, secretMountPath)
			}
			mountPath = secretMountPath
		}
	}
	if mountPath == "" {
		mountPath = "/"
	}
	return mountPath, nil
}
