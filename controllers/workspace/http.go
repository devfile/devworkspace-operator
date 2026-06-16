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

var httpClientsHolder HttpClientsHolder

type HttpClientsHolder interface {
	GetHttpClient() *http.Client
	GetHealthCheckHttpClient() *http.Client
	ConfigureHttpClients(context.Context, *controller.RoutingConfig)
}

type DefaultHttpClientsHolder struct {
	k8s    client.Client
	logger logr.Logger

	client                *http.Client
	healthCheckHttpClient *http.Client

	mu sync.RWMutex

	lastProxyConfig    *controller.Proxy
	lastCertsCMVersion string

	defaultCertPool *x509.CertPool
}

func NewDefaultHttpClientsHolder(k8s client.Client, logger logr.Logger) *DefaultHttpClientsHolder {
	defaultCertPool, err := x509.SystemCertPool()
	if err != nil {
		logger.Error(err, "Failed to load system cert pool")
		defaultCertPool = x509.NewCertPool()
	}

	clientsHolder := &DefaultHttpClientsHolder{
		k8s:             k8s,
		logger:          logger,
		defaultCertPool: defaultCertPool,
	}

	clientsHolder.ConfigureHttpClients(context.Background(), config.GetGlobalConfig().Routing)

	return clientsHolder
}

func (h *DefaultHttpClientsHolder) GetHttpClient() *http.Client {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.client
}

func (h *DefaultHttpClientsHolder) GetHealthCheckHttpClient() *http.Client {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.healthCheckHttpClient
}

func (h *DefaultHttpClientsHolder) ConfigureHttpClients(ctx context.Context, routingConfig *controller.RoutingConfig) {
	var newProxyConfig *controller.Proxy
	var newCertsCM *corev1.ConfigMap

	if routingConfig != nil {
		if routingConfig.ProxyConfig != nil {
			newProxyConfig = routingConfig.ProxyConfig
		}
		if routingConfig.TLSCertificateConfigmapRef != nil {
			certsCM, err := h.readCertCM(ctx, routingConfig.TLSCertificateConfigmapRef)
			if err != nil {
				h.logger.Error(err, "Failed to read TLS certificate ConfigMap")
			}

			newCertsCM = certsCM
		}
	}

	buildNewHttpClient, buildNewHealthCheckHttpClient := h.shouldRebuildClients(newProxyConfig, newCertsCM)

	if buildNewHttpClient || buildNewHealthCheckHttpClient {
		newClient, newHealthCheckClient := h.buildNewClients(
			buildNewHttpClient,
			buildNewHealthCheckHttpClient,
			newProxyConfig,
			newCertsCM,
		)

		h.setNewClients(
			newClient,
			newHealthCheckClient,
			newProxyConfig,
			newCertsCM,
		)
	}
}

func (h *DefaultHttpClientsHolder) shouldRebuildClients(newProxyConfig *controller.Proxy, newCertsCM *corev1.ConfigMap) (bool, bool) {
	defer h.mu.RUnlock()
	h.mu.RLock()

	// Always rebuild if clients haven't been initialized yet
	if h.client == nil || h.healthCheckHttpClient == nil {
		return true, true
	}

	if !reflect.DeepEqual(newProxyConfig, h.lastProxyConfig) {
		return true, true
	}

	certsCMVersion := ""
	if newCertsCM != nil {
		certsCMVersion = newCertsCM.ResourceVersion
	}

	if certsCMVersion != h.lastCertsCMVersion {
		return true, false
	}

	return false, false
}

func (h *DefaultHttpClientsHolder) buildNewClients(
	buildNewHttpClient bool,
	buildNewHealthCheckHttpClient bool,
	newProxyConfig *controller.Proxy,
	newCertsCM *corev1.ConfigMap,
) (*http.Client, *http.Client) {

	var newClient *http.Client
	var newHealthCheckClient *http.Client

	proxyFunc := h.getProxyFunc(newProxyConfig)
	caCertPool := h.getCaCertPool(newCertsCM)

	if buildNewHttpClient {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = proxyFunc
		transport.TLSClientConfig = &tls.Config{
			RootCAs: caCertPool,
		}

		newClient = &http.Client{
			Transport: transport,
		}
	}

	if buildNewHealthCheckHttpClient {
		healthCheckTransport := http.DefaultTransport.(*http.Transport).Clone()
		healthCheckTransport.Proxy = proxyFunc
		healthCheckTransport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}

		newHealthCheckClient = &http.Client{
			Transport: healthCheckTransport,
			Timeout:   500 * time.Millisecond,
		}
	}

	return newClient, newHealthCheckClient
}

func (h *DefaultHttpClientsHolder) setNewClients(
	newClient *http.Client,
	newHealthCheckClient *http.Client,
	newProxyConfig *controller.Proxy,
	newCertsCM *corev1.ConfigMap,
) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if newClient != nil {
		h.client = newClient
	}

	if newHealthCheckClient != nil {
		h.healthCheckHttpClient = newHealthCheckClient
	}

	if newProxyConfig != nil {
		h.lastProxyConfig = newProxyConfig.DeepCopy()
	} else {
		h.lastProxyConfig = nil
	}

	if newCertsCM != nil {
		h.lastCertsCMVersion = newCertsCM.ResourceVersion
	} else {
		h.lastCertsCMVersion = ""
	}
}

func (h *DefaultHttpClientsHolder) getProxyFunc(proxyConfig *controller.Proxy) func(*http.Request) (*url.URL, error) {
	if proxyConfig != nil {
		proxyConf := httpproxy.Config{}
		if proxyConfig.HttpProxy != nil {
			proxyConf.HTTPProxy = *proxyConfig.HttpProxy
		}
		if proxyConfig.HttpsProxy != nil {
			proxyConf.HTTPSProxy = *proxyConfig.HttpsProxy
		}
		if proxyConfig.NoProxy != nil {
			proxyConf.NoProxy = *proxyConfig.NoProxy
		}

		return func(req *http.Request) (*url.URL, error) {
			return proxyConf.ProxyFunc()(req.URL)
		}
	}

	return nil
}

func (h *DefaultHttpClientsHolder) getCaCertPool(cm *corev1.ConfigMap) *x509.CertPool {
	if cm == nil {
		return nil
	}

	caCertPool := h.defaultCertPool.Clone()

	for _, certsPem := range cm.Data {
		if !caCertPool.AppendCertsFromPEM([]byte(certsPem)) {
			h.logger.V(1).Info("Warning: failed to parse one or more certificates from ConfigMap")
		}
	}

	return caCertPool
}

func (h *DefaultHttpClientsHolder) readCertCM(ctx context.Context, cmReference *controller.ConfigmapReference) (*corev1.ConfigMap, error) {
	if cmReference == nil {
		return nil, nil
	}

	namespacedName := types.NamespacedName{
		Name:      cmReference.Name,
		Namespace: cmReference.Namespace,
	}

	configMap := &corev1.ConfigMap{}
	if err := h.k8s.Get(ctx, namespacedName, configMap); err != nil {
		return nil, fmt.Errorf("failed to read ConfigMap %s/%s containing certificates: %w", cmReference.Namespace, cmReference.Name, err)
	}

	return configMap, nil
}
