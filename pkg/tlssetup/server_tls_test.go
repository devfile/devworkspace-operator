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
	"context"
	"crypto/tls"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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

	err := RegisterSecurityProfileWatcher(nil, ServerTLS{}, nil, log)
	if err != nil {
		t.Errorf("RegisterSecurityProfileWatcher() on Kubernetes should be no-op, got error = %v", err)
	}
}

func TestRegisterSecurityProfileWatcher_ProfileNotFetched(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	defer infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	log := zap.New(zap.UseDevMode(true))

	err := RegisterSecurityProfileWatcher(nil, ServerTLS{}, nil, log)
	if err != nil {
		t.Errorf("RegisterSecurityProfileWatcher() with empty TLSOpts should skip setup, got error = %v", err)
	}
}

func TestBuildServerTLSOptions_NonOpenShift(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	defer infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	log := zap.New(zap.UseDevMode(true))
	ctx := context.Background()

	result, err := BuildServerTLSOptions(ctx, nil, nil, log, nil)
	if err != nil {
		t.Fatalf("Unexpected error on non-OpenShift: %v", err)
	}

	if result.TLSOpts != nil {
		t.Errorf("Expected nil TLSOpts on non-OpenShift, got %v", result.TLSOpts)
	}
	if result.profileFetched {
		t.Errorf("Expected profileFetched=false on non-OpenShift, got true")
	}
}

func TestBuildServerTLSOptions_OpenShift_LegacyAdherence(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	defer infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	log := zap.New(zap.UseDevMode(true))
	ctx := context.Background()

	scheme := runtime.NewScheme()
	if err := configv1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add configv1 to scheme: %v", err)
	}

	apiServer := &configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
		Spec: configv1.APIServerSpec{
			TLSSecurityProfile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						MinTLSVersion: configv1.VersionTLS12,
						Ciphers: []string{
							"TLS_AES_128_GCM_SHA256",
							"TLS_AES_256_GCM_SHA384",
						},
					},
				},
			},
			TLSAdherence: configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(apiServer).
		Build()

	result, err := BuildServerTLSOptions(ctx, nil, scheme, log, fakeClient)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result.profileFetched {
		t.Errorf("Expected profileFetched=true, got false")
	}
	if result.TLSOpts != nil {
		t.Errorf("Expected nil TLSOpts with legacy adherence policy, got %v", result.TLSOpts)
	}
	if result.InitialTLSAdherencePolicy != configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly {
		t.Errorf("Expected adherence policy %v, got %v",
			configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly,
			result.InitialTLSAdherencePolicy)
	}
	if result.InitialTLSProfileSpec.MinTLSVersion != configv1.VersionTLS12 {
		t.Errorf("Expected MinTLSVersion TLS12, got %v", result.InitialTLSProfileSpec.MinTLSVersion)
	}
}

func TestBuildServerTLSOptions_OpenShift_StrictAdherence(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	defer infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	log := zap.New(zap.UseDevMode(true))
	ctx := context.Background()

	scheme := runtime.NewScheme()
	if err := configv1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add configv1 to scheme: %v", err)
	}

	apiServer := &configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
		Spec: configv1.APIServerSpec{
			TLSSecurityProfile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						MinTLSVersion: configv1.VersionTLS12,
						Ciphers: []string{
							"TLS_AES_128_GCM_SHA256",
							"TLS_AES_256_GCM_SHA384",
						},
					},
				},
			},
			TLSAdherence: configv1.TLSAdherencePolicyStrictAllComponents,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(apiServer).
		Build()

	result, err := BuildServerTLSOptions(ctx, nil, scheme, log, fakeClient)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result.profileFetched {
		t.Errorf("Expected profileFetched=true, got false")
	}
	if result.TLSOpts == nil {
		t.Errorf("Expected non-nil TLSOpts with strict adherence policy, got nil")
	}
	if len(result.TLSOpts) != 1 {
		t.Errorf("Expected 1 TLSOpts function, got %d", len(result.TLSOpts))
	}
	if result.InitialTLSAdherencePolicy != configv1.TLSAdherencePolicyStrictAllComponents {
		t.Errorf("Expected adherence policy %v, got %v",
			configv1.TLSAdherencePolicyStrictAllComponents,
			result.InitialTLSAdherencePolicy)
	}

	if len(result.TLSOpts) > 0 {
		tlsConfig := &tls.Config{}
		result.TLSOpts[0](tlsConfig)
		if tlsConfig.MinVersion != tls.VersionTLS12 {
			t.Errorf("Expected MinVersion TLS12, got %v", tlsConfig.MinVersion)
		}
	}
}

func TestBuildServerTLSOptions_OpenShift_EmptyAdherence(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	defer infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	log := zap.New(zap.UseDevMode(true))
	ctx := context.Background()

	scheme := runtime.NewScheme()
	if err := configv1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add configv1 to scheme: %v", err)
	}

	apiServer := &configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
		Spec: configv1.APIServerSpec{
			TLSSecurityProfile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						MinTLSVersion: configv1.VersionTLS12,
					},
				},
			},
			// TLSAdherence not set (empty)
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(apiServer).
		Build()

	result, err := BuildServerTLSOptions(ctx, nil, scheme, log, fakeClient)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result.profileFetched {
		t.Errorf("Expected profileFetched=true, got false")
	}
	if result.TLSOpts != nil {
		t.Errorf("Expected nil TLSOpts with empty adherence policy, got %v", result.TLSOpts)
	}
}

func TestBuildServerTLSOptions_OpenShift_NoAPIServer(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	defer infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	log := zap.New(zap.UseDevMode(true))
	ctx := context.Background()

	scheme := runtime.NewScheme()
	if err := configv1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add configv1 to scheme: %v", err)
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	_, err := BuildServerTLSOptions(ctx, nil, scheme, log, fakeClient)

	if err == nil {
		t.Errorf("Expected error when APIServer resource is missing, got nil")
	}
}
