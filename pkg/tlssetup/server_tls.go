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
	"fmt"

	"github.com/go-logr/logr"
	configv1 "github.com/openshift/api/config/v1"
	ostls "github.com/openshift/controller-runtime-common/pkg/tls"
	libgocrypto "github.com/openshift/library-go/pkg/crypto"
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

// BuildServerTLSOptions fetches TLS profile and adherence policy from cluster.
// Returns TLS config functions when adherence policy requires strict compliance.
// Falls back to the library-go default TLS profile on transient API server errors
// or when adherence policy does not require strict compliance.
// Returns empty ServerTLS on non-OpenShift clusters.
func BuildServerTLSOptions(ctx context.Context, cfg *rest.Config, scheme *k8sruntime.Scheme, log logr.Logger) (ServerTLS, error) {
	if !infrastructure.IsOpenShift() {
		return ServerTLS{}, nil
	}

	cl, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return ServerTLS{}, fmt.Errorf("failed to create client for TLS profile fetch: %w", err)
	}

	return buildServerTLSOptions(ctx, cl, log)
}

func buildServerTLSOptions(ctx context.Context, cl client.Client, log logr.Logger) (ServerTLS, error) {
	var profile configv1.TLSProfileSpec
	var adherence configv1.TLSAdherencePolicy

	apiServer := &configv1.APIServer{}
	if err := cl.Get(ctx, client.ObjectKey{Name: ostls.APIServerName}, apiServer); err != nil {
		log.Error(err, "failed to read APIServer/cluster, falling back to library-go default TLS profile")
	} else if p, err := ostls.GetTLSProfileSpec(apiServer.Spec.TLSSecurityProfile); err != nil {
		log.Error(err, "failed to resolve TLS profile spec, falling back to library-go default TLS profile")
	} else {
		profile = p
		adherence = apiServer.Spec.TLSAdherence
	}

	serverTLS := ServerTLS{
		InitialTLSProfileSpec:     profile,
		InitialTLSAdherencePolicy: adherence,
	}

	if libgocrypto.ShouldHonorClusterTLSProfile(adherence) {
		tlsConfigFn, unsupported := ostls.NewTLSConfigFromProfile(profile)
		if len(unsupported) > 0 {
			log.Info("TLS profile contains ciphers unsupported by Go", "unsupported", unsupported)
		}

		if len(profile.Ciphers) > 0 && len(unsupported) == len(profile.Ciphers) {
			log.Error(nil, "no ciphers from the cluster TLS profile are supported by Go; server will use library-go defaults, which may not satisfy tlsAdherence",
				"profileCiphers", profile.Ciphers)
		}

		serverTLS.TLSOpts = []func(*tls.Config){tlsConfigFn}

		log.Info("Applying cluster TLS profile to metrics and webhook servers",
			"minTLSVersion", profile.MinTLSVersion)
		log.V(1).Info("TLS cipher list from cluster profile", "ciphers", profile.Ciphers)
	} else {
		defaultProfile := *configv1.TLSProfiles[libgocrypto.DefaultTLSProfileType]
		defaultTLSConfigFn, unsupported := ostls.NewTLSConfigFromProfile(defaultProfile)
		if len(unsupported) > 0 {
			log.Info("Default TLS profile contains ciphers unsupported by Go", "unsupported", unsupported)
		}

		serverTLS.TLSOpts = []func(*tls.Config){defaultTLSConfigFn}

		log.Info("Using library-go default TLS profile",
			"minTLSVersion", defaultProfile.MinTLSVersion,
			"adherencePolicy", adherence)
	}

	return serverTLS, nil
}

// RegisterSecurityProfileWatcher watches the APIServer TLS profile and adherence policy.
// Calls onCancel to trigger restart when either changes. No-op on non-OpenShift.
func RegisterSecurityProfileWatcher(mgr manager.Manager, serverTLS ServerTLS, onCancel context.CancelFunc, log logr.Logger) error {
	if !infrastructure.IsOpenShift() {
		return nil
	}

	watcher := &ostls.SecurityProfileWatcher{
		Client:                    mgr.GetClient(),
		InitialTLSProfileSpec:     serverTLS.InitialTLSProfileSpec,
		InitialTLSAdherencePolicy: serverTLS.InitialTLSAdherencePolicy,
		OnProfileChange: func(_ context.Context, _, newSpec configv1.TLSProfileSpec) {
			if !libgocrypto.ShouldHonorClusterTLSProfile(serverTLS.InitialTLSAdherencePolicy) {
				log.V(1).Info("Cluster TLS profile changed but adherence policy is not strict, not restarting")
				return
			}

			log.V(1).Info("TLS security profile changed, restarting operator", "minTLSVersion", newSpec.MinTLSVersion)
			onCancel()
		},
		OnAdherencePolicyChange: func(_ context.Context, _, newPolicy configv1.TLSAdherencePolicy) {
			log.V(1).Info("TLS adherence policy changed, restarting operator", "adherencePolicy", newPolicy)
			onCancel()
		},
	}

	return watcher.SetupWithManager(mgr)
}
