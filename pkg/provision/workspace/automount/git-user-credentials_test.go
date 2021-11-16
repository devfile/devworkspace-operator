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
	"testing"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestUserCredentialsAreMountedWithOneCredential(t *testing.T) {
	mountPath := "/sample/test"
	clusterAPI := sync.ClusterAPI{
		Client: fake.NewClientBuilder().Build(),
		Logger: zap.New(),
	}
	podAdditions, err := provisionUserGitCredentials(clusterAPI, testNamespace, mountPath, []string{
		"my_credentials",
	})
	if !assert.NoError(t, err, "Should not return error") {
		return
	}
	expectedAdditions := &v1alpha1.PodAdditions{}
	expectedAdditions.VolumeMounts = append(expectedAdditions.VolumeMounts, getGitCredentialsVolumeMount(mountPath, gitCredentialsSecretName))
	expectedAdditions.Volumes = append(expectedAdditions.Volumes, GetAutoMountVolumeWithSecret(gitCredentialsSecretName))
	assert.Equal(t, podAdditions, expectedAdditions)
}

func TestUserCredentialsAreOnlyMountedOnceWithMultipleCredentials(t *testing.T) {
	mountPath := "/sample/test"
	clusterAPI := sync.ClusterAPI{
		Client: fake.NewClientBuilder().Build(),
		Logger: zap.New(),
	}
	podAdditions, err := provisionUserGitCredentials(clusterAPI, testNamespace, mountPath, []string{
		"my_credentials",
		"my_other_credentials",
	})
	if !assert.NoError(t, err, "Should not return error") {
		return
	}
	expectedAdditions := &v1alpha1.PodAdditions{}
	expectedAdditions.VolumeMounts = append(expectedAdditions.VolumeMounts, getGitCredentialsVolumeMount(mountPath, gitCredentialsSecretName))
	expectedAdditions.Volumes = append(expectedAdditions.Volumes, GetAutoMountVolumeWithSecret(gitCredentialsSecretName))
	assert.Equal(t, podAdditions, expectedAdditions)
}
