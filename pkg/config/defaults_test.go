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
		assert.Equal(t, defaultKubernetesIngressPolicyRules, ingress)
		assert.Equal(t, defaultEgressPolicyRules, egress)
	})

	t.Run("on OpenShift", func(t *testing.T) {
		setupForTest(t)
		infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

		ingress, egress, err := GetDefaultNetworkPolicy("test-namespace")
		assert.NoError(t, err)
		assert.Len(t, ingress, len(defaultOpenShiftIngressPolicyRules)+1)
		assert.Equal(t, defaultEgressPolicyRules, egress)
	})
}
