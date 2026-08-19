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
//

// Package tlssetup builds TLS options for controller-runtime servers from the
// OpenShift APIServer TLS profile.
package tlssetup

import (
	"context"
	"crypto/tls"

	"github.com/go-logr/logr"
	configv1 "github.com/openshift/api/config/v1"
	ostls "github.com/openshift/controller-runtime-common/pkg/tls"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
)

// ServerTLS holds TLS options and the initial APIServer profile/adherence policy for watching.
type ServerTLS struct {
	TLSOpts                   []func(*tls.Config)
	InitialTLSProfileSpec     configv1.TLSProfileSpec
	InitialTLSAdherencePolicy configv1.TLSAdherencePolicy
}

// ShouldHonorClusterTLSProfile returns true when the component must honor the cluster
// TLS profile. Unknown values return true for forward compatibility.
func ShouldHonorClusterTLSProfile(adherence configv1.TLSAdherencePolicy) bool {
	switch adherence {
	case "", configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly:
		return false
	default:
		// StrictAllComponents or unknown future value → honor the profile
		return true
	}
}

// BuildServerTLSOptions fetches TLS settings from the OpenShift API server.
// Only applies the cluster profile when the tlsAdherence policy requires it.
func BuildServerTLSOptions(ctx context.Context, cfg *rest.Config, scheme *k8sruntime.Scheme, log logr.Logger) (ServerTLS, error) {
	var result ServerTLS

	if !infrastructure.IsOpenShift() {
		log.Info("Not running on OpenShift; using Go default TLS configuration")
		return result, nil
	}

	bootstrapClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		log.Error(err, "Failed to create bootstrap client for TLS profile fetch; using Go default TLS configuration")
		return result, nil
	}

	profile, err := ostls.FetchAPIServerTLSProfile(ctx, bootstrapClient)
	if err != nil {
		log.Error(err, "Failed to fetch TLS profile from APIServer; using Go default TLS configuration")
		return result, nil
	}

	adherence, err := ostls.FetchAPIServerTLSAdherencePolicy(ctx, bootstrapClient)
	if err != nil {
		log.Error(err, "Failed to fetch TLS adherence policy from APIServer; using Go default TLS configuration")
		return result, nil
	}

	result.InitialTLSProfileSpec = profile
	result.InitialTLSAdherencePolicy = adherence

	// Check if we should honor the cluster TLS profile
	if !ShouldHonorClusterTLSProfile(adherence) {
		log.Info("TLS adherence policy does not require strict adherence; using Go default TLS configuration",
			"policy", adherence)
		return result, nil
	}

	// Apply the cluster TLS profile
	tlsConfigFn, unsupported := ostls.NewTLSConfigFromProfile(profile)
	if len(unsupported) > 0 {
		log.Info("TLS profile contains ciphers unsupported by Go; they will be ignored",
			"unsupportedCiphers", unsupported)
	}

	result.TLSOpts = []func(*tls.Config){tlsConfigFn}

	log.Info("Applying cluster TLS profile to metrics and webhook servers",
		"minTLSVersion", profile.MinTLSVersion,
		"cipherCount", len(profile.Ciphers),
		"adherencePolicy", adherence)

	return result, nil
}

// RegisterSecurityProfileWatcher watches the APIServer TLS profile and adherence policy.
// Calls onCancel to trigger restart when either changes. No-op on non-OpenShift.
func RegisterSecurityProfileWatcher(mgr manager.Manager, serverTLS ServerTLS, onCancel context.CancelFunc, log logr.Logger) error {
	if !infrastructure.IsOpenShift() {
		return nil
	}

	// Only set up the watcher if we successfully fetched the initial profile
	if len(serverTLS.TLSOpts) == 0 {
		log.Info("Skipping TLS profile watcher (profile not applied)")
		return nil
	}

	watcher := &ostls.SecurityProfileWatcher{
		Client:                    mgr.GetClient(),
		InitialTLSProfileSpec:     serverTLS.InitialTLSProfileSpec,
		InitialTLSAdherencePolicy: serverTLS.InitialTLSAdherencePolicy,
		OnProfileChange: func(_ context.Context, old, new configv1.TLSProfileSpec) {
			log.Info("TLS security profile changed; initiating graceful restart",
				"oldMinTLSVersion", old.MinTLSVersion,
				"newMinTLSVersion", new.MinTLSVersion)
			onCancel()
		},
		OnAdherencePolicyChange: func(_ context.Context, old, new configv1.TLSAdherencePolicy) {
			log.Info("TLS adherence policy changed; initiating graceful restart",
				"old", old,
				"new", new)
			onCancel()
		},
	}

	return watcher.SetupWithManager(mgr)
}
