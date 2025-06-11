//
// Copyright (c) 2019-2025 Red Hat, Inc.
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

package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestGetPVCSpecWithDefaultStorageAccessMode(t *testing.T) {
	// Given
	name := "test-pvc"
	namespace := "default"
	storageClass := "standard"

	// When
	pvc, err := getPVCSpec(name, namespace, &storageClass, resource.MustParse("5Gi"), nil)

	// Then
	assert.NoError(t, err, "Expected no error")
	assert.Equal(t, name, pvc.Name, "PVC name should match")
	assert.Equal(t, namespace, pvc.Namespace, "PVC namespace should match")
	assert.Equal(t, storageClass, *pvc.Spec.StorageClassName, "Storage class should match")
	assert.Equal(t, []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, pvc.Spec.AccessModes, "Access modes should match")
	assert.Equal(t, "5Gi", pvc.Spec.Resources.Requests.Storage().String(), "Storage size should match")
}

func TestGetPVCSpecWithCustomStorageAccessMode(t *testing.T) {
	// Given
	name := "test-pvc"
	namespace := "default"
	storageClass := "standard"
	storageAccessMode := []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOncePod}

	// When
	pvc, err := getPVCSpec(name, namespace, &storageClass, resource.MustParse("5Gi"), storageAccessMode)

	// Then
	assert.NoError(t, err, "Expected no error")
	assert.Equal(t, name, pvc.Name, "PVC name should match")
	assert.Equal(t, namespace, pvc.Namespace, "PVC namespace should match")
	assert.Equal(t, storageClass, *pvc.Spec.StorageClassName, "Storage class should match")
	assert.Equal(t, []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOncePod}, pvc.Spec.AccessModes, "Access modes should match")
	assert.Equal(t, "5Gi", pvc.Spec.Resources.Requests.Storage().String(), "Storage size should match")
}
