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

func TestPodTemplateMetadataDiffFunc(t *testing.T) {
	tests := []struct {
		name          string
		specLabels    map[string]string
		specAnns      map[string]string
		clusterLabels map[string]string
		clusterAnns   map[string]string
		expectUpdate  bool
	}{
		{
			name:          "no diff when labels match",
			specLabels:    map[string]string{"app": "test"},
			clusterLabels: map[string]string{"app": "test"},
			expectUpdate:  false,
		},
		{
			name:          "no diff when cluster has extra labels",
			specLabels:    map[string]string{"app": "test"},
			clusterLabels: map[string]string{"app": "test", "paas.redhat.com/appcode": "ITOS-123"},
			expectUpdate:  false,
		},
		{
			name:          "diff when spec label missing from cluster",
			specLabels:    map[string]string{"app": "test", "new-label": "value"},
			clusterLabels: map[string]string{"app": "test"},
			expectUpdate:  true,
		},
		{
			name:          "diff when spec label value differs",
			specLabels:    map[string]string{"app": "test-v2"},
			clusterLabels: map[string]string{"app": "test-v1"},
			expectUpdate:  true,
		},
		{
			name:         "no diff when cluster has extra annotations",
			specAnns:     map[string]string{"note": "hello"},
			clusterAnns:  map[string]string{"note": "hello", "external.io/injected": "true"},
			expectUpdate: false,
		},
		{
			name:         "diff when spec annotation missing from cluster",
			specAnns:     map[string]string{"note": "hello"},
			clusterAnns:  map[string]string{},
			expectUpdate: true,
		},
		{
			name:          "no diff with nil maps",
			specLabels:    nil,
			clusterLabels: nil,
			expectUpdate:  false,
		},
		{
			name:          "no diff when cluster has labels but spec has none",
			specLabels:    nil,
			clusterLabels: map[string]string{"external": "label"},
			expectUpdate:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels:      tt.specLabels,
							Annotations: tt.specAnns,
						},
					},
				},
			}
			cluster := &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels:      tt.clusterLabels,
							Annotations: tt.clusterAnns,
						},
					},
				},
			}

			_, update := podTemplateMetadataDiffFunc(spec, cluster)
			if update != tt.expectUpdate {
				t.Errorf("podTemplateMetadataDiffFunc() update = %v, want %v", update, tt.expectUpdate)
			}
		})
	}
}

func TestPodTemplateMetadataDiffFunc_NonDeployment(t *testing.T) {
	spec := &corev1.ConfigMap{}
	cluster := &corev1.ConfigMap{}
	shouldDelete, shouldUpdate := podTemplateMetadataDiffFunc(spec, cluster)
	if shouldDelete || shouldUpdate {
		t.Errorf("podTemplateMetadataDiffFunc() should return (false, false) for non-Deployment types, got (%v, %v)", shouldDelete, shouldUpdate)
	}
}

func TestDeploymentDiffOpts_IgnoresPodTemplateMetadata(t *testing.T) {
	spec := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "main",
						Image: "test:latest",
					}},
				},
			},
		},
	}
	cluster := spec.DeepCopy()
	cluster.Spec.Template.Labels["external.io/injected"] = "true"

	diffFn := basicDiffFunc(deploymentDiffOpts)
	_, update := diffFn(spec, cluster)
	if update {
		t.Error("basicDiffFunc(deploymentDiffOpts) should not detect extra pod template labels as a diff")
	}
}

func TestDeploymentDiffOpts_DetectsSpecChanges(t *testing.T) {
	spec := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "main",
						Image: "test:v2",
					}},
				},
			},
		},
	}
	cluster := spec.DeepCopy()
	cluster.Spec.Template.Spec.Containers[0].Image = "test:v1"

	diffFn := basicDiffFunc(deploymentDiffOpts)
	_, update := diffFn(spec, cluster)
	if !update {
		t.Error("basicDiffFunc(deploymentDiffOpts) should detect container image changes as a diff")
	}
}

func TestDeploymentFullDiff_ExternalLabelsNoUpdate(t *testing.T) {
	specLabels := map[string]string{
		"controller.devfile.io/devworkspace_id":   "workspace123",
		"controller.devfile.io/devworkspace_name": "my-workspace",
	}
	spec := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels: specLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"controller.devfile.io/devworkspace_id": "workspace123",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "main",
						Image: "test:latest",
					}},
				},
			},
		},
	}
	cluster := spec.DeepCopy()
	cluster.Labels["paas.redhat.com/appcode"] = "ITOS-123"
	cluster.Spec.Template.Labels["paas.redhat.com/appcode"] = "ITOS-123"

	deploymentDiff := diffFuncs[reflect.TypeOf(appsv1.Deployment{})]
	_, update := deploymentDiff(spec, cluster)
	if update {
		t.Error("deployment diff should not trigger update when only external labels are added to deployment and pod template metadata")
	}
}
