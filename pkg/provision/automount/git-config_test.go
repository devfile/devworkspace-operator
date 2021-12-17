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
	"testing"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	corev1 "k8s.io/api/core/v1"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	defaultName      = "sample-configmap"
	defaultMountPath = "/sample"
	defaultData      = map[string]string{
		hostKey:        "github.com",
		certificateKey: "sample_data_here",
	}
	testNamespace = "test-namespace"
)

func TestOneConfigMapWithNoUserMountPath(t *testing.T) {
	mountPath := ""
	clusterConfig := buildConfig(defaultName, mountPath, defaultData)
	clusterAPI := sync.ClusterAPI{
		Client: fake.NewClientBuilder().WithObjects(clusterConfig).Build(),
		Logger: zap.New(),
	}
	_, gitconfig, err := constructGitConfig(clusterAPI, testNamespace, mountPath)
	if !assert.NoError(t, err, "Should not return error") {
		return
	}
	assert.NotContains(t, gitconfig, "[credential]")
}

func TestOneConfigMapWithMountPathAndHostAndCert(t *testing.T) {
	mountPath := "/sample/test"
	clusterConfig := buildConfig(defaultName, mountPath, defaultData)
	clusterAPI := sync.ClusterAPI{
		Client: fake.NewClientBuilder().WithObjects(clusterConfig).Build(),
		Logger: zap.New(),
	}
	_, gitconfig, err := constructGitConfig(clusterAPI, testNamespace, mountPath)
	if !assert.NoError(t, err, "Should not return error") {
		return
	}
	assert.Contains(t, gitconfig, fmt.Sprintf(gitServerTemplate, defaultData["host"], filepath.Join(mountPath, certificateKey)))
}

func TestOneConfigMapWithMountPathAndWithoutHostAndWithoutCert(t *testing.T) {
	mountPath := "/sample/test"
	clusterConfig := buildConfig(defaultName, mountPath, map[string]string{})
	clusterAPI := sync.ClusterAPI{
		Client: fake.NewClientBuilder().WithObjects(clusterConfig).Build(),
		Logger: zap.New(),
	}
	_, _, err := constructGitConfig(clusterAPI, testNamespace, mountPath)
	assert.Equal(t, err.Error(), fmt.Sprintf("could not find certificate field in configmap %s", defaultName))
}

func TestOneConfigMapWithMountPathAndWithoutHostAndWithCert(t *testing.T) {
	mountPath := "/sample/test"
	clusterConfig := buildConfig(defaultName, mountPath, map[string]string{
		certificateKey: "test_cert_data",
	})
	clusterAPI := sync.ClusterAPI{
		Client: fake.NewClientBuilder().WithObjects(clusterConfig).Build(),
		Logger: zap.New(),
	}
	_, gitconfig, err := constructGitConfig(clusterAPI, testNamespace, mountPath)
	if !assert.NoError(t, err, "Should not return error") {
		return
	}
	assert.Contains(t, gitconfig, fmt.Sprintf(defaultGitServerTemplate, filepath.Join(mountPath, certificateKey)))
}

func TestOneConfigMapWithMountPathAndWithHostAndWithoutCert(t *testing.T) {
	mountPath := "/sample/test"
	clusterConfig := buildConfig(defaultName, mountPath, map[string]string{
		hostKey: "some_host",
	})
	clusterAPI := sync.ClusterAPI{
		Client: fake.NewClientBuilder().WithObjects(clusterConfig).Build(),
		Logger: zap.New(),
	}
	_, _, err := constructGitConfig(clusterAPI, testNamespace, mountPath)
	assert.Equal(t, err.Error(), fmt.Sprintf("could not find certificate field in configmap %s", defaultName))
}

func TestOneConfigMapWithNoDefinedMountPathInAnnotation(t *testing.T) {
	mountPath := "/sample/test"
	clusterConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultName,
			Namespace: testNamespace,
			Labels: map[string]string{
				constants.DevWorkspaceGitTLSLabel: "true",
			},
		},
		Data: defaultData,
	}
	clusterAPI := sync.ClusterAPI{
		Client: fake.NewClientBuilder().WithObjects(clusterConfig).Build(),
		Logger: zap.New(),
	}
	_, _, err := constructGitConfig(clusterAPI, testNamespace, mountPath)
	assert.Equal(t, err.Error(), fmt.Sprintf("could not find mount path in configmap %s", defaultName))
}

func TestTwoConfigMapWithNoDefinedMountPathInAnnotation(t *testing.T) {
	config1 := buildConfig("configmap1", "/folder1", defaultData)
	config2 := buildConfig("configmap2", "/folder2", defaultData)
	clusterAPI := sync.ClusterAPI{
		Client: fake.NewClientBuilder().WithObjects(config1, config2).Build(),
		Logger: zap.New(),
	}
	_, gitconfig, err := constructGitConfig(clusterAPI, testNamespace, "")
	if !assert.NoError(t, err, "Should not return error") {
		return
	}
	assert.Equal(t, gitconfig, "[http \"github.com\"]\n    sslCAInfo = /folder1/certificate\n\n[http \"github.com\"]\n    sslCAInfo = /folder2/certificate\n\n")
}

func TestTwoConfigMapWithOneDefaultTLSAndOtherGithubTLS(t *testing.T) {
	config1 := buildConfig("configmap1", "/folder1", map[string]string{
		certificateKey: "sample_data_here",
	})
	config2 := buildConfig("configmap2", "/folder2", map[string]string{
		hostKey:        "github.com",
		certificateKey: "sample_data_here",
	})

	clusterAPI := sync.ClusterAPI{
		Client: fake.NewClientBuilder().WithObjects(config1, config2).Build(),
		Logger: zap.New(),
	}
	_, gitconfig, err := constructGitConfig(clusterAPI, testNamespace, "")
	if !assert.NoError(t, err, "Should not return error") {
		return
	}
	assert.Equal(t, gitconfig, "[http]\n    sslCAInfo = /folder1/certificate\n\n[http \"github.com\"]\n    sslCAInfo = /folder2/certificate\n\n")
}

func TestTwoConfigMapWithBothMissingHost(t *testing.T) {
	config1 := buildConfig("configmap1", "/folder1", map[string]string{
		certificateKey: "sample_data_here",
	})
	config2 := buildConfig("configmap2", "/folder2", map[string]string{
		certificateKey: "sample_data_here",
	})

	clusterAPI := sync.ClusterAPI{
		Client: fake.NewClientBuilder().WithObjects(config1, config2).Build(),
		Logger: zap.New(),
	}
	_, _, err := constructGitConfig(clusterAPI, testNamespace, "")
	assert.Equal(t, err.Error(), "multiple git tls credentials do not have host specified")
}

func TestGitConfigIsFullyMounted(t *testing.T) {
	defaultConfig := buildConfig(defaultName, defaultMountPath, defaultData)
	clusterAPI := sync.ClusterAPI{
		Client: fake.NewClientBuilder().WithObjects(defaultConfig).Build(),
		Logger: zap.New(),
	}
	podAdditions, err := provisionGitConfig(clusterAPI, testNamespace, defaultMountPath)
	if !assert.NoError(t, err, "Should not return error") {
		return
	}

	expectedAdditions := &v1alpha1.PodAdditions{}
	expectedAdditions.Volumes = append(expectedAdditions.Volumes, getAutoMountVolumeWithConfigMap(defaultName), getAutoMountVolumeWithConfigMap(gitCredentialsConfigMapName))
	expectedAdditions.VolumeMounts = append(expectedAdditions.VolumeMounts, getAutoMountConfigMapVolumeMount(defaultMountPath, defaultName), getGitConfigMapVolumeMount(gitCredentialsConfigMapName))
	assert.Equal(t, podAdditions, expectedAdditions, fmt.Sprintf("Processed config should merge settings from cluster: %s", cmp.Diff(podAdditions, expectedAdditions)))
}

func buildConfig(name string, mountPath string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
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
