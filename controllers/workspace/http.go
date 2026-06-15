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

	"k8s.io/apimachinery/pkg/types"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"golang.org/x/net/http/httpproxy"
)

type HttpClientsHolder interface {
	GetHttpClient(routingConfig *controller.RoutingConfig) *http.Client
	GetHealthCheckHttpClient(routingConfig *controller.RoutingConfig) *http.Client
}

type DefaultHttpsClientHolder struct {
	k8s    client.Client
	logger logr.Logger

	client                *http.Client
	healthCheckHttpClient *http.Client
}

var httpClientsHolder HttpClientsHolder

func NewDefaultHttpsClientHolder(k8s client.Client, logger logr.Logger) *DefaultHttpsClientHolder {
	return &DefaultHttpsClientHolder{k8s: k8s, logger: logger}
}

func (h *DefaultHttpsClientHolder) GetHttpClient(routingConfig *controller.RoutingConfig) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = h.getProxyFunc(routingConfig)
	transport.TLSClientConfig = &tls.Config{
		RootCAs: h.getCaCertPool(routingConfig),
	}

	return &http.Client{
		Transport: transport,
	}
}

func (h *DefaultHttpsClientHolder) GetHealthCheckHttpClient(routingConfig *controller.RoutingConfig) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = h.getProxyFunc(routingConfig)
	transport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   500 * time.Millisecond,
	}
}

func (h *DefaultHttpsClientHolder) getProxyFunc(routingConfig *controller.RoutingConfig) func(*http.Request) (*url.URL, error) {
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

func (h *DefaultHttpsClientHolder) getCaCertPool(routingConfig *controller.RoutingConfig) *x509.CertPool {
	if certs, ok := h.readCertificates(routingConfig); ok {
		var caCertPool *x509.CertPool

		systemCertPool, err := x509.SystemCertPool()
		if err != nil {
			h.logger.Error(err, "Failed to load system cert pool")
			caCertPool = x509.NewCertPool()
		} else {
			caCertPool = systemCertPool
		}

		for _, certsPem := range certs {
			caCertPool.AppendCertsFromPEM([]byte(certsPem))
		}

		return caCertPool
	}

	return nil
}

func (h *DefaultHttpsClientHolder) readCertificates(routingConfig *controller.RoutingConfig) (map[string]string, bool) {
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
