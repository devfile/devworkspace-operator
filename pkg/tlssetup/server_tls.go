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

// Package tlssetup builds TLS options for controller-runtime metrics and webhook servers from
// the OpenShift APIServer TLS profile and optional CLI overrides.
package tlssetup

import (
	"context"
	"crypto/tls"
	"strings"

	"github.com/go-logr/logr"
	configv1 "github.com/openshift/api/config/v1"
	ostls "github.com/openshift/controller-runtime-common/pkg/tls"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
)

// ServerTLS holds TLS options and, on OpenShift, the APIServer profile used to watch for changes.
type ServerTLS struct {
	TLSOpts                   []func(*tls.Config)
	InitialTLSProfileSpec     configv1.TLSProfileSpec
	InitialTLSAdherencePolicy configv1.TLSAdherencePolicy
	// CLITLSOverride is true when tls-min-version or tls-cipher-suites flags were set.
	CLITLSOverride bool
	// openShiftTLSFetched is true when the TLS profile was successfully retrieved from the
	// OpenShift APIServer. If false, the SecurityProfileWatcher is not registered.
	openShiftTLSFetched bool
}

// BuildServerTLSOptions loads TLS settings for secure serving: on OpenShift, from the cluster
// APIServer resource unless CLI flags override; with CLI flags, those always win.
// If the APIServer TLS config is not accessible on OpenShift, a warning is logged and the
// function falls back to the original behaviour (no custom TLS options applied).
func BuildServerTLSOptions(ctx context.Context, cfg *rest.Config, scheme *k8sruntime.Scheme, tlsMinVersion, tlsCipherSuites string, log logr.Logger) (ServerTLS, error) {
	var result ServerTLS
	result.CLITLSOverride = tlsMinVersion != "" || tlsCipherSuites != ""

	if infrastructure.IsOpenShift() {
		bootstrapClient, err := client.New(cfg, client.Options{Scheme: scheme})
		if err != nil {
			log.Error(err, "Failed to create bootstrap client for TLS profile fetch; falling back to default TLS configuration")
		} else {
			profileSpec, err := ostls.FetchAPIServerTLSProfile(ctx, bootstrapClient)
			if err != nil {
				log.Error(err, "Failed to fetch TLS profile from APIServer; falling back to default TLS configuration")
			} else {
				adherencePolicy, err := ostls.FetchAPIServerTLSAdherencePolicy(ctx, bootstrapClient)
				if err != nil {
					log.Error(err, "Failed to fetch TLS adherence policy from APIServer; falling back to default TLS configuration")
				} else {
					result.InitialTLSProfileSpec = profileSpec
					result.InitialTLSAdherencePolicy = adherencePolicy
					result.openShiftTLSFetched = true
					log.Info("Fetched TLS profile from APIServer",
						"minTLSVersion", profileSpec.MinTLSVersion, "ciphers", profileSpec.Ciphers)

					if !result.CLITLSOverride {
						tlsConfigFunc, unsupportedCiphers := ostls.NewTLSConfigFromProfile(profileSpec)
						if len(unsupportedCiphers) > 0 {
							log.Info("Ignoring unsupported cipher suites from cluster TLS profile",
								"ciphers", unsupportedCiphers)
						}
						result.TLSOpts = []func(*tls.Config){tlsConfigFunc}
					}
				}
			}
		}
	}

	if result.CLITLSOverride {
		customSpec := configv1.TLSProfileSpec{
			MinTLSVersion: configv1.TLSProtocolVersion(tlsMinVersion),
		}
		if tlsCipherSuites != "" {
			customSpec.Ciphers = strings.Split(tlsCipherSuites, ",")
		}
		tlsConfigFunc, unsupportedCiphers := ostls.NewTLSConfigFromProfile(customSpec)
		if len(unsupportedCiphers) > 0 {
			log.Info("Ignoring unsupported cipher suites from CLI flags", "ciphers", unsupportedCiphers)
		}
		result.TLSOpts = []func(*tls.Config){tlsConfigFunc}
		log.Info("Using CLI-provided TLS configuration (overrides cluster TLS profile)")
	}

	return result, nil
}

// RegisterSecurityProfileWatcher watches TLS profile changes and triggers restart via onCancel.
// No-op on non-OpenShift, when CLI override is set, or when profile fetch failed.
func RegisterSecurityProfileWatcher(mgr manager.Manager, serverTLS ServerTLS, onCancel context.CancelFunc, log logr.Logger) error {
	if !infrastructure.IsOpenShift() || serverTLS.CLITLSOverride || !serverTLS.openShiftTLSFetched {
		return nil
	}
	watcher := &ostls.SecurityProfileWatcher{
		Client:                    mgr.GetClient(),
		InitialTLSProfileSpec:     serverTLS.InitialTLSProfileSpec,
		InitialTLSAdherencePolicy: serverTLS.InitialTLSAdherencePolicy,
		OnProfileChange: func(watchCtx context.Context, old, new configv1.TLSProfileSpec) {
			log.Info("TLS security profile changed; initiating graceful restart to apply new profile",
				"oldMinTLSVersion", old.MinTLSVersion, "newMinTLSVersion", new.MinTLSVersion)
			onCancel()
		},
		OnAdherencePolicyChange: func(watchCtx context.Context, old, new configv1.TLSAdherencePolicy) {
			log.Info("TLS adherence policy changed; initiating graceful restart to apply new policy",
				"old", old, "new", new)
			onCancel()
		},
	}
	return watcher.SetupWithManager(mgr)
}
