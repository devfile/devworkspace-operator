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

package controllers

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"sync"
	"time"

	controller "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"k8s.io/apimachinery/pkg/types"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"golang.org/x/net/http/httpproxy"
)

var httpClientsFactory HttpClientsFactory

type HttpClientsFactory interface {
	// GetHttpClient returns an HTTP client configured with proxy, TLS, and custom CA certificates
	// from routingConfig.
	GetHttpClient(context.Context, *controller.RoutingConfig) *http.Client

	// GetHealthCheckHttpClient returns an HTTP client that skips TLS verification.
	// This client MUST only be used for workspace health/readiness checks, not for
	// fetching external content or making security-sensitive requests.
	GetHealthCheckHttpClient() *http.Client
}

// DefaultHttpClientsFactory is a thread-safe, caching implementation of HttpClientsFactory.
// It caches one HTTP client and one health-check client, rebuilding either only when the
// relevant routing configuration (proxy settings, TLS certificates) changes.
type DefaultHttpClientsFactory struct {
	k8s    client.Client
	logger logr.Logger

	httpClient            *http.Client
	healthCheckHttpClient *http.Client

	mu sync.RWMutex

	httpClientProxyConfig  *controller.Proxy
	httpClientConfigmapRef *controller.ConfigmapReference
	httpClientCertsVersion string

	systemCertPool *x509.CertPool
	proxyFunc      func(*http.Request) (*url.URL, error)
}

func SetupHttpClientsFactory(k8s client.Client, logger logr.Logger) error {
	systemCertPool, err := x509.SystemCertPool()
	if err != nil {
		return fmt.Errorf("failed to load system cert pool: %w", err)
	}

	proxyFunc := getProxyFunc()

	healthCheckHttpClientTransport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyFunc != nil {
		// Preserve the default proxy (from env vars) when no explicit proxy is configured.
		healthCheckHttpClientTransport.Proxy = proxyFunc
	}
	healthCheckHttpClientTransport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	httpClientsFactory = &DefaultHttpClientsFactory{
		k8s:            k8s,
		logger:         logger,
		systemCertPool: systemCertPool,
		proxyFunc:      proxyFunc,
		healthCheckHttpClient: &http.Client{
			Transport: healthCheckHttpClientTransport,
			Timeout:   500 * time.Millisecond,
		},
	}

	return nil
}

func (h *DefaultHttpClientsFactory) GetHttpClient(ctx context.Context, routingConfig *controller.RoutingConfig) *http.Client {
	certsCM := h.readCertificates(ctx, routingConfig)

	h.mu.RLock()
	if !h.shouldCreateHttpClient(routingConfig, certsCM) {
		defer h.mu.RUnlock()
		return h.httpClient
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.shouldCreateHttpClient(routingConfig, certsCM) {
		h.httpClient = h.createHttpClient(certsCM)

		if routingConfig == nil {
			h.httpClientProxyConfig = nil
		} else {
			h.httpClientProxyConfig = routingConfig.ProxyConfig.DeepCopy()
		}

		if certsCM == nil {
			h.httpClientCertsVersion = ""
			h.httpClientConfigmapRef = nil
		} else {
			h.httpClientCertsVersion = certsCM.ResourceVersion
			h.httpClientConfigmapRef = &controller.ConfigmapReference{
				Name:      certsCM.Name,
				Namespace: certsCM.Namespace,
			}
		}
	}

	return h.httpClient
}

func (h *DefaultHttpClientsFactory) createHttpClient(certsCM *corev1.ConfigMap) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if h.proxyFunc != nil {
		// Preserve the default proxy (from env vars) when no explicit proxy is configured.
		transport.Proxy = h.proxyFunc
	}
	transport.TLSClientConfig = &tls.Config{
		RootCAs: h.getCaCertPool(certsCM),
	}

	return &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}
}

func (h *DefaultHttpClientsFactory) shouldCreateHttpClient(routingConfig *controller.RoutingConfig, certsCM *corev1.ConfigMap) bool {
	if h.httpClient == nil {
		return true
	}

	var certsVersion string
	var configmapRef *controller.ConfigmapReference
	var proxyConfig *controller.Proxy

	if certsCM != nil {
		certsVersion = certsCM.ResourceVersion
		configmapRef = &controller.ConfigmapReference{
			Name:      certsCM.Name,
			Namespace: certsCM.Namespace,
		}
	}

	if routingConfig != nil {
		proxyConfig = routingConfig.ProxyConfig
	}

	return certsVersion != h.httpClientCertsVersion ||
		!reflect.DeepEqual(configmapRef, h.httpClientConfigmapRef) ||
		!reflect.DeepEqual(proxyConfig, h.httpClientProxyConfig)
}

func (h *DefaultHttpClientsFactory) GetHealthCheckHttpClient() *http.Client {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.healthCheckHttpClient
}

// getProxyFunc returns a proxy function based on the proxy settings in routingConfig.
func getProxyFunc() func(*http.Request) (*url.URL, error) {
	globalConfig := config.GetGlobalConfig()

	if globalConfig.Routing != nil && globalConfig.Routing.ProxyConfig != nil {
		proxyConf := httpproxy.Config{}
		if globalConfig.Routing.ProxyConfig.HttpProxy != nil {
			proxyConf.HTTPProxy = *globalConfig.Routing.ProxyConfig.HttpProxy
		}
		if globalConfig.Routing.ProxyConfig.HttpsProxy != nil {
			proxyConf.HTTPSProxy = *globalConfig.Routing.ProxyConfig.HttpsProxy
		}
		if globalConfig.Routing.ProxyConfig.NoProxy != nil {
			proxyConf.NoProxy = *globalConfig.Routing.ProxyConfig.NoProxy
		}

		return func(req *http.Request) (*url.URL, error) {
			return proxyConf.ProxyFunc()(req.URL)
		}
	}

	return nil
}

// getCaCertPool returns a CA cert pool that includes system certs and any additional certs from the ConfigMap.
// A nil pool causes the HTTP client to use the system default root CAs.
func (h *DefaultHttpClientsFactory) getCaCertPool(certsCM *corev1.ConfigMap) *x509.CertPool {
	if certsCM == nil || len(certsCM.Data) == 0 {
		return nil
	}

	caCertPool := h.systemCertPool.Clone()

	for _, certsPem := range certsCM.Data {
		if !caCertPool.AppendCertsFromPEM([]byte(certsPem)) {
			h.logger.Error(fmt.Errorf("failed to parse one or more certificates from ConfigMap"), "Could not append CA certificates to pool")
		}
	}

	return caCertPool
}

func (h *DefaultHttpClientsFactory) readCertificates(ctx context.Context, routingConfig *controller.RoutingConfig) *corev1.ConfigMap {
	if routingConfig == nil || routingConfig.TLSCertificateConfigmapRef == nil {
		return nil
	}

	configmapRef := routingConfig.TLSCertificateConfigmapRef

	namespacedName := types.NamespacedName{
		Name:      configmapRef.Name,
		Namespace: configmapRef.Namespace,
	}

	configMap := &corev1.ConfigMap{}
	if err := h.k8s.Get(ctx, namespacedName, configMap); err != nil {
		// print and ignore the error, http clients will be created with host's root CA set.
		h.logger.Error(err, "Failed to read ConfigMap containing certificates", "namespace", configmapRef.Namespace, "name", configmapRef.Name)
		return nil
	}

	return configMap
}
