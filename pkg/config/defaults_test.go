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

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
)

func TestGetDefaultNetworkPolicyDoesNotMutateDefaultConfig(t *testing.T) {
	setupForTest(t)

	defaultConfig.Workspace.NetworkPolicy = nil

	_, _, err := GetDefaultNetworkPolicy("test-namespace")
	assert.NoError(t, err)

	assert.Nil(t, defaultConfig.Workspace.NetworkPolicy, "GetDefaultNetworkPolicy should not mutate defaultConfig")
}

func TestGetDefaultNetworkPolicy(t *testing.T) {
	t.Run("on Kubernetes", func(t *testing.T) {
		setupForTest(t)
		infrastructure.InitializeForTesting(infrastructure.Kubernetes)

		ingress, egress, err := GetDefaultNetworkPolicy("test-namespace")
		assert.NoError(t, err)
		assert.Equal(t, []networkingv1.NetworkPolicyIngressRule{{}}, ingress)
		assert.Equal(t, []networkingv1.NetworkPolicyEgressRule{{}}, egress)
	})

	t.Run("on OpenShift", func(t *testing.T) {
		setupForTest(t)
		infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

		ingress, egress, err := GetDefaultNetworkPolicy("test-namespace")
		assert.NoError(t, err)
		assert.Len(t, ingress, len(defaultOpenShiftIngressPolicyRules)+1)
		assert.Equal(t, defaultOpenShiftIngressPolicyRules, ingress[1:])
		assert.Equal(t, []networkingv1.NetworkPolicyPeer{
			{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"kubernetes.io/metadata.name": "test-namespace"},
				},
				PodSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app.kubernetes.io/part-of": "devworkspace-operator"},
				},
			},
		}, ingress[0].From)
		assert.Equal(t, []networkingv1.NetworkPolicyEgressRule{{}}, egress)
	})
}

func TestGetDefaultNetworkPolicyReturnsCopies(t *testing.T) {
	tests := map[string]infrastructure.Type{
		"on Kubernetes": infrastructure.Kubernetes,
		"on OpenShift":  infrastructure.OpenShiftv4,
	}
	for name, infraType := range tests {
		t.Run(name, func(t *testing.T) {
			setupForTest(t)
			infrastructure.InitializeForTesting(infraType)

			origKubernetesIngress := (&networkingv1.NetworkPolicySpec{Ingress: defaultKubernetesIngressPolicyRules}).DeepCopy().Ingress
			origOpenShiftIngress := (&networkingv1.NetworkPolicySpec{Ingress: defaultOpenShiftIngressPolicyRules}).DeepCopy().Ingress
			origEgress := (&networkingv1.NetworkPolicySpec{Egress: defaultEgressPolicyRules}).DeepCopy().Egress

			ingress, egress, err := GetDefaultNetworkPolicy("test-namespace")
			assert.NoError(t, err)

			for i := range ingress {
				for _, peer := range ingress[i].From {
					if peer.NamespaceSelector != nil {
						peer.NamespaceSelector.MatchLabels["mutated"] = "true"
					}
					if peer.PodSelector != nil {
						peer.PodSelector.MatchLabels["mutated"] = "true"
					}
				}
				ingress[i].From = append(ingress[i].From, networkingv1.NetworkPolicyPeer{})
			}
			for i := range egress {
				egress[i].To = append(egress[i].To, networkingv1.NetworkPolicyPeer{})
			}

			assert.Equal(t, origKubernetesIngress, defaultKubernetesIngressPolicyRules)
			assert.Equal(t, origOpenShiftIngress, defaultOpenShiftIngressPolicyRules)
			assert.Equal(t, origEgress, defaultEgressPolicyRules)
		})
	}
}
