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

package workspace

import (
	"testing"
	"time"

	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/devfile/devworkspace-operator/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/stretchr/testify/assert"
)

func TestPullSecrets_TableDriven(t *testing.T) {
	namespace := "test-ns"
	serviceAccountName := "test-sa"

	tests := []struct {
		name            string
		objects         []client.Object
		setupInfra      func()
		expectedError   error
		expectedSecrets []string // expected PullSecret names
	}{
		{
			name:    "ServiceAccount not found",
			objects: []client.Object{}, // No SA created
			setupInfra: func() {
				infrastructure.InitializeForTesting(infrastructure.Kubernetes)
			},
			expectedError:   nil,
			expectedSecrets: []string{},
		},
		{
			name: "Secret with incorrect type is skipped",
			objects: []client.Object{
				&corev1.ServiceAccount{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ServiceAccount",
						APIVersion: corev1.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      serviceAccountName,
						Namespace: namespace,
					},
				},
				&corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: corev1.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bad-secret",
						Namespace: namespace,
						Labels: map[string]string{
							constants.DevWorkspacePullSecretLabel: "true",
						},
					},
					Type: corev1.SecretTypeOpaque, // Not a docker config type
				},
			},
			setupInfra: func() {
				infrastructure.InitializeForTesting(infrastructure.Kubernetes)
			},
			expectedError:   nil,
			expectedSecrets: []string{},
		},
		{
			name: "RetryError when OpenShift SA is too new with no secrets",
			objects: []client.Object{
				&corev1.ServiceAccount{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ServiceAccount",
						APIVersion: corev1.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:              serviceAccountName,
						Namespace:         namespace,
						CreationTimestamp: metav1.NewTime(time.Now()), // Recent
					},
				},
			},
			setupInfra: func() {
				infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
			},
			expectedError:   &dwerrors.RetryError{},
			expectedSecrets: nil,
		},
		{
			name: "Non-OpenShift: no retry even if SA is recent with no secrets",
			objects: []client.Object{
				&corev1.ServiceAccount{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ServiceAccount",
						APIVersion: corev1.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:              serviceAccountName,
						Namespace:         namespace,
						CreationTimestamp: metav1.NewTime(time.Now()),
					},
				},
			},
			setupInfra: func() {
				infrastructure.InitializeForTesting(infrastructure.Kubernetes)
			},
			expectedError:   nil,
			expectedSecrets: []string{},
		},
		{
			name: "Multiple SA + labeled secrets merged and sorted",
			objects: []client.Object{
				&corev1.ServiceAccount{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ServiceAccount",
						APIVersion: corev1.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:              serviceAccountName,
						Namespace:         namespace,
						CreationTimestamp: metav1.NewTime(time.Now().Add(-10 * time.Minute)),
					},
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "z-sa-secret"},
					},
				},
				&corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: corev1.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "a-labeled-secret",
						Namespace: namespace,
						Labels: map[string]string{
							constants.DevWorkspacePullSecretLabel: "true",
						},
					},
					Type: corev1.SecretTypeDockerConfigJson,
				},
			},
			setupInfra: func() {
				infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
			},
			expectedError:   nil,
			expectedSecrets: []string{"a-labeled-secret", "z-sa-secret"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			scheme := runtime.NewScheme()
			assert.NoError(t, corev1.AddToScheme(scheme))
			tt.setupInfra()

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				Build()

			clusterAPI := sync.ClusterAPI{
				Client: fakeClient,
			}

			// When
			result, err := PullSecrets(clusterAPI, serviceAccountName, namespace)

			// Then
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.IsType(t, tt.expectedError, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, result)
			actualNames := []string{}
			for _, ps := range result.PullSecrets {
				actualNames = append(actualNames, ps.Name)
			}
			assert.Equal(t, tt.expectedSecrets, actualNames)
		})
	}
}
