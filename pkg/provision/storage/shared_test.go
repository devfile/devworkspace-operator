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
	"context"
	"testing"

	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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

func TestNodeAffinityForHostname(t *testing.T) {
	affinity := NodeAffinityForHostname("worker-node-1")

	assert.NotNil(t, affinity)
	assert.NotNil(t, affinity.NodeAffinity)
	nodeSelector := affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
	assert.NotNil(t, nodeSelector)
	assert.Len(t, nodeSelector.NodeSelectorTerms, 1)
	assert.Len(t, nodeSelector.NodeSelectorTerms[0].MatchExpressions, 1)

	expr := nodeSelector.NodeSelectorTerms[0].MatchExpressions[0]
	assert.Equal(t, "kubernetes.io/hostname", expr.Key)
	assert.Equal(t, corev1.NodeSelectorOpIn, expr.Operator)
	assert.Equal(t, []string{"worker-node-1"}, expr.Values)
}

func TestFindNodeForPVC(t *testing.T) {
	pvcVolume := func(claimName string) corev1.Volume {
		return corev1.Volume{
			Name: "workspace-data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: claimName,
				},
			},
		}
	}
	makePod := func(name, nodeName, namespace string, phase corev1.PodPhase, volumes ...corev1.Volume) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    map[string]string{constants.DevWorkspaceIDLabel: "some-id"},
			},
			Spec:   corev1.PodSpec{NodeName: nodeName, Volumes: volumes},
			Status: corev1.PodStatus{Phase: phase},
		}
	}

	const ns = "test-ns"

	tests := []struct {
		name     string
		pods     []*corev1.Pod
		pvcName  string
		expected string
	}{
		{
			name:     "returns node for running pod with matching PVC",
			pods:     []*corev1.Pod{makePod("pod-1", "node-1", ns, corev1.PodRunning, pvcVolume("claim-devworkspace"))},
			pvcName:  "claim-devworkspace",
			expected: "node-1",
		},
		{
			name:     "ignores non-running pods",
			pods:     []*corev1.Pod{makePod("pod-1", "node-1", ns, corev1.PodPending, pvcVolume("claim-devworkspace"))},
			pvcName:  "claim-devworkspace",
			expected: "",
		},
		{
			name:     "returns empty for non-matching PVC name",
			pods:     []*corev1.Pod{makePod("pod-1", "node-1", ns, corev1.PodRunning, pvcVolume("other-pvc"))},
			pvcName:  "claim-devworkspace",
			expected: "",
		},
		{
			name:     "returns empty when no pods exist",
			pods:     nil,
			pvcName:  "claim-devworkspace",
			expected: "",
		},
		{
			name: "returns first matching node when multiple pods mount same PVC",
			pods: []*corev1.Pod{
				makePod("pod-1", "node-1", ns, corev1.PodRunning, pvcVolume("claim-devworkspace")),
				makePod("pod-2", "node-2", ns, corev1.PodRunning, pvcVolume("claim-devworkspace")),
			},
			pvcName:  "claim-devworkspace",
			expected: "node-1",
		},
		{
			name: "ignores pods without PVC volumes",
			pods: []*corev1.Pod{
				makePod("pod-1", "node-1", ns, corev1.PodRunning, corev1.Volume{
					Name:         "config",
					VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{}},
				}),
			},
			pvcName:  "claim-devworkspace",
			expected: "",
		},
		{
			name: "only considers pods with DevWorkspaceIDLabel",
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "unlabeled-pod", Namespace: ns},
					Spec:       corev1.PodSpec{NodeName: "node-1", Volumes: []corev1.Volume{pvcVolume("claim-devworkspace")}},
					Status:     corev1.PodStatus{Phase: corev1.PodRunning},
				},
			},
			pvcName:  "claim-devworkspace",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(scheme)
			for _, p := range tt.pods {
				builder = builder.WithObjects(p)
			}
			fakeClient := builder.Build()

			result, err := FindNodeForPVC(context.Background(), fakeClient, ns, tt.pvcName)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
