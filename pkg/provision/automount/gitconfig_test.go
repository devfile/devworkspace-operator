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
	"path/filepath"
	"testing"
	"time"

	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	defaultName      = "sample-configmap"
	defaultMountPath = "/sample"
	defaultData      = map[string]string{
		gitTLSHostKey:        "github.com",
		gitTLSCertificateKey: "sample_data_here",
	}
)

func TestUserCredentialsAreMountedWithOneCredential(t *testing.T) {
	mountPath := "/sample/test"
	testSecret := buildSecret("test-secret", mountPath, map[string][]byte{
		gitCredentialsSecretKey: []byte("my_credentials"),
	})

	clusterAPI := sync.ClusterAPI{
		Client: fake.NewClientBuilder().WithObjects(&testSecret).Build(),
		Logger: zap.New(),
	}

	var resources *Resources
	// ProvisionGitConfiguration has to be called multiple times since it stops after creating each configmap/secret
	ok := assert.Eventually(t, func() bool {
		var err error
		resources, err = ProvisionGitConfiguration(clusterAPI, testNamespace)
		t.Log(err)
		return err == nil
	}, 100*time.Millisecond, 10*time.Millisecond)
	if ok {
		assert.Len(t, resources.Volumes, 2, "Should mount two volumes")
		assert.Len(t, resources.VolumeMounts, 2, "Should have two volumeMounts")
	}
}

func TestUserCredentialsAreOnlyMountedOnceWithMultipleCredentials(t *testing.T) {
	mountPath := "/sample/test"
	testSecret1 := buildSecret("test-secret-1", mountPath, map[string][]byte{
		gitCredentialsSecretKey: []byte("my_credentials"),
	})
	testSecret2 := buildSecret("test-secret-2", mountPath, map[string][]byte{
		gitCredentialsSecretKey: []byte("my_other_credentials"),
	})
	clusterAPI := sync.ClusterAPI{
		Client: fake.NewClientBuilder().WithObjects(&testSecret1, &testSecret2).Build(),
		Logger: zap.New(),
	}
	var resources *Resources
	// ProvisionGitConfiguration has to be called multiple times since it stops after creating each configmap/secret
	ok := assert.Eventually(t, func() bool {
		var err error
		resources, err = ProvisionGitConfiguration(clusterAPI, testNamespace)
		t.Log(err)
		return err == nil
	}, 100*time.Millisecond, 10*time.Millisecond)
	if ok {
		assert.Len(t, resources.Volumes, 2, "Should mount two volumes")
		assert.Len(t, resources.VolumeMounts, 2, "Should have two volumeMounts")
	}
}

func TestGitConfigIsFullyMounted(t *testing.T) {
	defaultConfig := buildConfig(defaultName, defaultMountPath, defaultData)
	clusterAPI := sync.ClusterAPI{
		Client: fake.NewClientBuilder().WithObjects(&defaultConfig).Build(),
		Logger: zap.New(),
	}
	var resources *Resources
	// ProvisionGitConfiguration has to be called multiple times since it stops after creating each configmap/secret
	ok := assert.Eventually(t, func() bool {
		var err error
		resources, err = ProvisionGitConfiguration(clusterAPI, testNamespace)
		t.Log(err)
		return err == nil
	}, 100*time.Millisecond, 10*time.Millisecond)
	if ok {
		assert.Len(t, resources.Volumes, 2, "Should mount two volumes")
		assert.Len(t, resources.VolumeMounts, 2, "Should have two volumeMounts")
	}
}

func TestOneConfigMapWithNoUserMountPath(t *testing.T) {
	mountPath := ""
	configmaps := []corev1.ConfigMap{
		buildConfig(defaultName, mountPath, defaultData),
	}

	gitconfig, err := constructGitConfig(testNamespace, mountPath, configmaps, nil)
	if !assert.NoError(t, err, "Should not return error") {
		return
	}
	assert.NotContains(t, gitconfig.Data[gitConfigName], "[credential]")
}

func TestOneConfigMapWithMountPathAndHostAndCert(t *testing.T) {
	mountPath := "/sample/test"
	configmaps := []corev1.ConfigMap{
		buildConfig(defaultName, mountPath, defaultData),
	}

	gitconfig, err := constructGitConfig(testNamespace, mountPath, configmaps, nil)
	if !assert.NoError(t, err, "Should not return error") {
		return
	}
	assert.Contains(t, gitconfig.Data[gitConfigName], fmt.Sprintf(gitServerTemplate, defaultData[gitTLSHostKey], filepath.Join(mountPath, gitTLSCertificateKey)))
}

func TestOneConfigMapWithMountPathAndWithoutHostAndWithoutCert(t *testing.T) {
	mountPath := "/sample/test"
	configmaps := []corev1.ConfigMap{
		buildConfig(defaultName, mountPath, map[string]string{}),
	}

	_, err := constructGitConfig(testNamespace, mountPath, configmaps, nil)
	assert.Equal(t, err.Error(), fmt.Sprintf("could not find certificate field in configmap %s", defaultName))
}

func TestOneConfigMapWithMountPathAndWithoutHostAndWithCert(t *testing.T) {
	mountPath := "/sample/test"
	configmaps := []corev1.ConfigMap{
		buildConfig(defaultName, mountPath, map[string]string{
			gitTLSCertificateKey: "test_cert_data",
		}),
	}

	gitconfig, err := constructGitConfig(testNamespace, mountPath, configmaps, nil)
	if !assert.NoError(t, err, "Should not return error") {
		return
	}
	assert.Contains(t, gitconfig.Data[gitConfigName], fmt.Sprintf(defaultGitServerTemplate, filepath.Join(mountPath, gitTLSCertificateKey)))
}

func TestOneConfigMapWithMountPathAndWithHostAndWithoutCert(t *testing.T) {
	mountPath := "/sample/test"
	configmaps := []corev1.ConfigMap{
		buildConfig(defaultName, mountPath, map[string]string{
			gitTLSHostKey: "some_host",
		}),
	}

	_, err := constructGitConfig(testNamespace, mountPath, configmaps, nil)
	assert.Equal(t, err.Error(), fmt.Sprintf("could not find certificate field in configmap %s", defaultName))
}

func TestTwoConfigMapWithNoDefinedMountPathInAnnotation(t *testing.T) {
	configmaps := []corev1.ConfigMap{
		buildConfig("configmap1", "/folder1", defaultData),
		buildConfig("configmap2", "/folder2", defaultData),
	}

	gitconfig, err := constructGitConfig(testNamespace, "", configmaps, nil)
	if !assert.NoError(t, err, "Should not return error") {
		return
	}
	expectedGitConfig := fmt.Sprintf("%s\n[http \"github.com\"]\n    sslCAInfo = /folder1/certificate\n\n[http \"github.com\"]\n    sslCAInfo = /folder2/certificate\n", gitLFSConfig)
	assert.Equal(t, gitconfig.Data[gitConfigName], expectedGitConfig)
}

func TestTwoConfigMapWithOneDefaultTLSAndOtherGithubTLS(t *testing.T) {
	configmaps := []corev1.ConfigMap{
		buildConfig("configmap1", "/folder1", map[string]string{
			gitTLSCertificateKey: "sample_data_here",
		}),
		buildConfig("configmap2", "/folder2", map[string]string{
			gitTLSHostKey:        "github.com",
			gitTLSCertificateKey: "sample_data_here",
		}),
	}

	gitconfig, err := constructGitConfig(testNamespace, "", configmaps, nil)
	if !assert.NoError(t, err, "Should not return error") {
		return
	}
	expectedGitConfig := fmt.Sprintf("%s\n[http]\n    sslCAInfo = /folder1/certificate\n\n[http \"github.com\"]\n    sslCAInfo = /folder2/certificate\n", gitLFSConfig)
	assert.Equal(t, gitconfig.Data[gitConfigName], expectedGitConfig)
}

func TestTwoConfigMapWithBothMissingHost(t *testing.T) {
	configmaps := []corev1.ConfigMap{
		buildConfig("configmap1", "/folder1", map[string]string{
			gitTLSCertificateKey: "sample_data_here",
		}),
		buildConfig("configmap2", "/folder2", map[string]string{
			gitTLSCertificateKey: "sample_data_here",
		}),
	}

	_, err := constructGitConfig(testNamespace, "", configmaps, nil)
	assert.Equal(t, err.Error(), "multiple git tls credentials do not have host specified")
}

func buildConfig(name string, mountPath string, data map[string]string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
			Labels: map[string]string{
				constants.DevWorkspaceGitTLSLabel: "true",
			},
			Annotations: map[string]string{
				constants.DevWorkspaceMountPathAnnotation: mountPath,
			},
		},
		Data: data,
	}
}

func buildSecret(name string, mountPath string, data map[string][]byte) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
			Labels: map[string]string{
				constants.DevWorkspaceGitCredentialLabel: "true",
			},
			Annotations: map[string]string{
				constants.DevWorkspaceMountPathAnnotation: mountPath,
			},
		},
		Data: data,
	}
}
