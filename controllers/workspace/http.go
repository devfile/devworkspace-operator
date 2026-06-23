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
	GetHealthCheckHttpClient(*controller.RoutingConfig) *http.Client
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

	healthCheckHttpClientProxyConfig *controller.Proxy

	systemCertPool *x509.CertPool
}

func SetupHttpClientsFactory(k8s client.Client, logger logr.Logger) error {
	systemCertPool, err := x509.SystemCertPool()
	if err != nil {
		return fmt.Errorf("failed to load system cert pool: %w", err)
	}

	httpClientsFactory = &DefaultHttpClientsFactory{
		k8s:            k8s,
		logger:         logger,
		systemCertPool: systemCertPool,
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
		h.httpClient = h.createHttpClient(routingConfig, certsCM)

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

func (h *DefaultHttpClientsFactory) createHttpClient(routingConfig *controller.RoutingConfig, certsCM *corev1.ConfigMap) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	proxyFunc := h.getProxyFunc(routingConfig)
	if proxyFunc != nil {
		// If Proxy is nil or returns a nil *URL, no proxy is used.
		transport.Proxy = proxyFunc
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

func (h *DefaultHttpClientsFactory) GetHealthCheckHttpClient(routingConfig *controller.RoutingConfig) *http.Client {
	h.mu.RLock()
	if !h.shouldCreateHealthCheckHttpClient(routingConfig) {
		defer h.mu.RUnlock()
		return h.healthCheckHttpClient
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.shouldCreateHealthCheckHttpClient(routingConfig) {
		h.healthCheckHttpClient = h.createHealthCheckHttpClient(routingConfig)

		if routingConfig == nil {
			h.healthCheckHttpClientProxyConfig = nil
		} else {
			h.healthCheckHttpClientProxyConfig = routingConfig.ProxyConfig.DeepCopy()
		}
	}

	return h.healthCheckHttpClient
}

func (h *DefaultHttpClientsFactory) shouldCreateHealthCheckHttpClient(routingConfig *controller.RoutingConfig) bool {
	if h.healthCheckHttpClient == nil {
		return true
	}

	var proxyConfig *controller.Proxy

	if routingConfig != nil {
		proxyConfig = routingConfig.ProxyConfig
	}

	return !reflect.DeepEqual(proxyConfig, h.healthCheckHttpClientProxyConfig)
}

func (h *DefaultHttpClientsFactory) createHealthCheckHttpClient(routingConfig *controller.RoutingConfig) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	proxyFunc := h.getProxyFunc(routingConfig)
	if proxyFunc != nil {
		// If Proxy is nil or returns a nil *URL, no proxy is used.
		transport.Proxy = proxyFunc
	}
	transport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   500 * time.Millisecond,
	}
}

// getProxyFunc returns a proxy function based on the proxy settings in routingConfig.
// Returns nil if no proxy is configured; a nil proxy func causes the HTTP transport to
// use the default proxy settings from environment variables.
func (h *DefaultHttpClientsFactory) getProxyFunc(routingConfig *controller.RoutingConfig) func(*http.Request) (*url.URL, error) {
	if routingConfig == nil || routingConfig.ProxyConfig == nil {
		return nil
	}

	// Since routingConfig is the result of merging the global configuration with
	// the workspace configuration, we need an additional check to avoid accidentally
	// resetting the proxy configuration.
	if *routingConfig.ProxyConfig.HttpProxy == "" || *routingConfig.ProxyConfig.HttpsProxy == "" {
		return nil
	}

	proxyConfig := httpproxy.Config{}
	if routingConfig.ProxyConfig.HttpProxy != nil {
		proxyConfig.HTTPProxy = *routingConfig.ProxyConfig.HttpProxy
	}
	if routingConfig.ProxyConfig.HttpsProxy != nil {
		proxyConfig.HTTPSProxy = *routingConfig.ProxyConfig.HttpsProxy
	}
	if routingConfig.ProxyConfig.NoProxy != nil {
		proxyConfig.NoProxy = *routingConfig.ProxyConfig.NoProxy
	}

	return func(req *http.Request) (*url.URL, error) {
		return proxyConfig.ProxyFunc()(req.URL)
	}
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
