//
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

package tlssetup

import (
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
)

func TestShouldHonorClusterTLSProfile(t *testing.T) {
	tests := []struct {
		name      string
		adherence configv1.TLSAdherencePolicy
		expected  bool
	}{
		{
			name:      "Empty policy should not honor cluster TLS profile",
			adherence: "",
			expected:  false,
		},
		{
			name:      "LegacyAdheringComponentsOnly should not honor cluster TLS profile",
			adherence: configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly,
			expected:  false,
		},
		{
			name:      "StrictAllComponents should honor cluster TLS profile",
			adherence: configv1.TLSAdherencePolicyStrictAllComponents,
			expected:  true,
		},
		{
			name:      "Unknown policy should honor cluster TLS profile for forward compatibility",
			adherence: configv1.TLSAdherencePolicy("UnknownFuturePolicy"),
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldHonorClusterTLSProfile(tt.adherence)
			if got != tt.expected {
				t.Errorf("ShouldHonorClusterTLSProfile(%v) = %v, expected %v", tt.adherence, got, tt.expected)
			}
		})
	}
}

func TestRegisterSecurityProfileWatcher_NonOpenShift(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	defer infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	log := zap.New(zap.UseDevMode(true))

	// On non-OpenShift, should be a no-op and return nil
	err := RegisterSecurityProfileWatcher(nil, ServerTLS{}, nil, log)
	if err != nil {
		t.Errorf("RegisterSecurityProfileWatcher() on Kubernetes should be no-op, got error = %v", err)
	}
}

func TestRegisterSecurityProfileWatcher_NoTLSOpts(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	defer infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	log := zap.New(zap.UseDevMode(true))

	// When TLSOpts is empty (profile not applied), should skip watcher setup and return nil
	err := RegisterSecurityProfileWatcher(nil, ServerTLS{}, nil, log)
	if err != nil {
		t.Errorf("RegisterSecurityProfileWatcher() with empty TLSOpts should skip setup, got error = %v", err)
	}
}
