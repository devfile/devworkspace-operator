// Copyright (c) 2019-2026 Red Hat, Inc.
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

package sync

import (
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetUpdateFunc_Deployment(t *testing.T) {
	deploy := &appsv1.Deployment{}
	fn := getUpdateFunc(deploy)
	defaultFn := getUpdateFunc(&corev1.ConfigMap{})

	deployFnPtr := reflect.ValueOf(fn).Pointer()
	defaultFnPtr := reflect.ValueOf(defaultFn).Pointer()
	if deployFnPtr == defaultFnPtr {
		t.Error("getUpdateFunc should return deploymentUpdateFunc for Deployments, not defaultUpdateFunc")
	}

	spec := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
			},
		},
	}
	cluster := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			ResourceVersion: "456",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test", "paas.redhat.com/appcode": "ITOS-123"},
				},
			},
		},
	}

	result, err := fn(spec, cluster)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resultDeploy := result.(*appsv1.Deployment)
	expectedLabels := map[string]string{"app": "test"}
	if !reflect.DeepEqual(resultDeploy.Spec.Template.Labels, expectedLabels) {
		t.Errorf("pod template labels mismatch:\n  got:  %v\n  want: %v", resultDeploy.Spec.Template.Labels, expectedLabels)
	}
}

func TestDeploymentUpdateFunc(t *testing.T) {
	tests := []struct {
		name            string
		specLabels      map[string]string
		specAnnotations map[string]string
		clusterLabels   map[string]string
		clusterAnns     map[string]string
		expectedLabels  map[string]string
		expectedAnns    map[string]string
	}{
		{
			name:           "spec labels replace cluster labels",
			specLabels:     map[string]string{"app": "test"},
			clusterLabels:  map[string]string{"app": "test", "paas.redhat.com/appcode": "ITOS-123"},
			expectedLabels: map[string]string{"app": "test"},
		},
		{
			name:           "spec wins on conflict",
			specLabels:     map[string]string{"app": "new-value"},
			clusterLabels:  map[string]string{"app": "old-value", "external": "keep"},
			expectedLabels: map[string]string{"app": "new-value"},
		},
		{
			name:            "spec annotations replace cluster annotations",
			specAnnotations: map[string]string{"note": "from-spec"},
			clusterAnns:     map[string]string{"note": "from-spec", "injected": "by-webhook"},
			expectedAnns:    map[string]string{"note": "from-spec"},
		},
		{
			name:           "handles nil cluster labels",
			specLabels:     map[string]string{"app": "test"},
			clusterLabels:  nil,
			expectedLabels: map[string]string{"app": "test"},
		},
		{
			name:           "handles nil spec labels",
			specLabels:     nil,
			clusterLabels:  map[string]string{"external": "keep"},
			expectedLabels: nil,
		},
		{
			name:           "removed spec label does not persist from cluster",
			specLabels:     map[string]string{"app": "test"},
			clusterLabels:  map[string]string{"app": "test", "env": "staging"},
			expectedLabels: map[string]string{"app": "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels:      tt.specLabels,
							Annotations: tt.specAnnotations,
						},
					},
				},
			}
			cluster := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test",
					ResourceVersion: "123",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels:      tt.clusterLabels,
							Annotations: tt.clusterAnns,
						},
					},
				},
			}

			result, err := deploymentUpdateFunc(spec, cluster)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			resultDeploy := result.(*appsv1.Deployment)
			if resultDeploy.ResourceVersion != "123" {
				t.Errorf("expected ResourceVersion '123', got '%s'", resultDeploy.ResourceVersion)
			}

			if !reflect.DeepEqual(resultDeploy.Spec.Template.Labels, tt.expectedLabels) {
				t.Errorf("labels mismatch:\n  got:  %v\n  want: %v", resultDeploy.Spec.Template.Labels, tt.expectedLabels)
			}
			if tt.expectedAnns != nil {
				if !reflect.DeepEqual(resultDeploy.Spec.Template.Annotations, tt.expectedAnns) {
					t.Errorf("annotations mismatch:\n  got:  %v\n  want: %v", resultDeploy.Spec.Template.Annotations, tt.expectedAnns)
				}
			}
		})
	}
}

func TestDeploymentUpdateFunc_NilCluster(t *testing.T) {
	spec := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
	}
	result, err := deploymentUpdateFunc(spec, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.GetName() != "test" {
		t.Errorf("expected name 'test', got '%s'", result.GetName())
	}
}
