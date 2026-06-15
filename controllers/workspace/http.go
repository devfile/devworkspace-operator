// Copyright (c) 2019-2025 Red Hat, Inc.
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
	"net/http"
	"net/url"
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
	ConfigureHttpClients(routingConfig *controller.RoutingConfig)
}

type DefaultHttpClientsHolder struct {
	k8s    client.Client
	logger logr.Logger

	client                *http.Client
	healthCheckHttpClient *http.Client

	defaultCertPool *x509.CertPool
}

func NewDefaultHttpClientsHolder(k8s client.Client, logger logr.Logger) *DefaultHttpClientsHolder {
	defaultCertPool, err := x509.SystemCertPool()
	if err != nil {
		logger.Error(err, "Failed to load system cert pool")
		defaultCertPool = x509.NewCertPool()
	}

	clientsHolder := &DefaultHttpClientsHolder{
		k8s:    k8s,
		logger: logger,

		defaultCertPool: defaultCertPool,
	}

	clientsHolder.setupHttpClients()
	clientsHolder.ConfigureHttpClients(config.GetGlobalConfig().Routing)

	return clientsHolder
}

func (h *DefaultHttpClientsHolder) GetHttpClient() *http.Client {
	return h.client
}

func (h *DefaultHttpClientsHolder) GetHealthCheckHttpClient() *http.Client {
	return h.healthCheckHttpClient
}

func (t *DefaultHttpClientsHolder) ConfigureHttpClients(routingConfig *controller.RoutingConfig) {
	proxyFunc := t.getProxyFunc(routingConfig)
	caCertPool := t.getCaCertPool(routingConfig)

	t.client.Transport.(*http.Transport).Proxy = proxyFunc
	t.client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{
		RootCAs: caCertPool,
	}

	t.healthCheckHttpClient.Transport.(*http.Transport).Proxy = proxyFunc
}

func (t *DefaultHttpClientsHolder) setupHttpClients() {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	t.client = &http.Client{
		Transport: transport,
	}

	healthCheckTransport := http.DefaultTransport.(*http.Transport).Clone()
	healthCheckTransport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	t.healthCheckHttpClient = &http.Client{
		Transport: healthCheckTransport,
		Timeout:   500 * time.Millisecond,
	}
}

func (h *DefaultHttpClientsHolder) getProxyFunc(routingConfig *controller.RoutingConfig) func(*http.Request) (*url.URL, error) {
	if routingConfig != nil && routingConfig.ProxyConfig != nil {
		proxyConf := httpproxy.Config{}
		if routingConfig.ProxyConfig.HttpProxy != nil {
			proxyConf.HTTPProxy = *routingConfig.ProxyConfig.HttpProxy
		}
		if routingConfig.ProxyConfig.HttpsProxy != nil {
			proxyConf.HTTPSProxy = *routingConfig.ProxyConfig.HttpsProxy
		}
		if routingConfig.ProxyConfig.NoProxy != nil {
			proxyConf.NoProxy = *routingConfig.ProxyConfig.NoProxy
		}

		return func(req *http.Request) (*url.URL, error) {
			return proxyConf.ProxyFunc()(req.URL)
		}
	}

	return nil
}

func (h *DefaultHttpClientsHolder) getCaCertPool(routingConfig *controller.RoutingConfig) *x509.CertPool {
	if certs, ok := h.readCertificates(routingConfig); ok {
		caCertPool := h.defaultCertPool.Clone()

		for _, certsPem := range certs {
			if !caCertPool.AppendCertsFromPEM([]byte(certsPem)) {
				h.logger.Info("Warning: failed to parse one or more certificates from ConfigMap")
			}
		}

		return caCertPool
	}

	return nil
}

func (h *DefaultHttpClientsHolder) readCertificates(routingConfig *controller.RoutingConfig) (map[string]string, bool) {
	if routingConfig == nil || routingConfig.TLSCertificateConfigmapRef == nil {
		return nil, false
	}

	configmapRef := routingConfig.TLSCertificateConfigmapRef

	namespacedName := types.NamespacedName{
		Name:      configmapRef.Name,
		Namespace: configmapRef.Namespace,
	}

	configMap := &corev1.ConfigMap{}
	err := h.k8s.Get(context.Background(), namespacedName, configMap)
	if err != nil {
		h.logger.Error(err, "Failed to read configmap with certificates")
		return nil, false
	}

	return configMap.Data, true
}
