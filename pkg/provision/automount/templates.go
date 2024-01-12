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

const gitCredentialsSecretKey = "credentials"

// gitLFSConfig is the default configuration that gets provisioned when git-lfs
// is installed. It needs to be included in the overridden gitconfig to avoid
// disabling git-lfs in repos that require a gitconfig.
const gitLFSConfig = `[filter "lfs"]
    clean = git-lfs clean -- %f
    smudge = git-lfs smudge -- %f
    process = git-lfs filter-process
    required = true
`

// Since we're mounting the credentials file read-only, we need to ignore 'store'
// and 'erase' commands to the credential helper (will print an error about being
// unable to get a lock on the credentials file). This snippet effectively wraps
// only 'git credential-store get' to make it read-only.
const credentialTemplate = `[credential]
    helper = "!f() { test \"$1\" = get && git credential-store --file %s \"$@\"; }; f"
`

const gitServerTemplate = `[http "%s"]
    sslCAInfo = %s
`

const defaultGitServerTemplate = `[http]
    sslCAInfo = %s
`

func constructGitConfig(namespace, credentialMountPath string, certificatesConfigMaps []corev1.ConfigMap, baseGitConfig *string) (*corev1.ConfigMap, error) {
	var configSettings []string
	configSettings = append(configSettings, gitLFSConfig)

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
			Name:      constants.GitCredentialsConfigMapName,
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
			Name:      constants.GitCredentialsMergedSecretName,
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
