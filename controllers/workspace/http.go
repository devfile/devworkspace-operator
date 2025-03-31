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

	"github.com/devfile/devworkspace-operator/pkg/config"

	"k8s.io/apimachinery/pkg/types"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"golang.org/x/net/http/httpproxy"
)

var (
	httpClient            *http.Client
	healthCheckHttpClient *http.Client
)

func setupHttpClients(k8s client.Client, logger logr.Logger) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	healthCheckTransport := http.DefaultTransport.(*http.Transport).Clone()
	healthCheckTransport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

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

		proxyFunc := func(req *http.Request) (*url.URL, error) {
			return proxyConf.ProxyFunc()(req.URL)
		}
		transport.Proxy = proxyFunc
		healthCheckTransport.Proxy = proxyFunc
	}

	httpClient = &http.Client{
		Transport: transport,
	}
	healthCheckHttpClient = &http.Client{
		Transport: healthCheckTransport,
		Timeout:   500 * time.Millisecond,
	}
	InjectCertificates(k8s, logger)
}

func InjectCertificates(k8s client.Client, logger logr.Logger) {
	if certs, ok := readCertificates(k8s, logger); ok {
		for _, certsPem := range certs {
			injectCertificates([]byte(certsPem), httpClient.Transport.(*http.Transport), logger)
		}
	}
}

func readCertificates(k8s client.Client, logger logr.Logger) (map[string]string, bool) {
	configmapRef := config.GetGlobalConfig().Routing.TLSCertificateConfigmapRef
	if configmapRef == nil {
		return nil, false
	}
	configMap := &corev1.ConfigMap{}
	namespacedName := &types.NamespacedName{
		Name:      configmapRef.Name,
		Namespace: configmapRef.Namespace,
	}
	err := k8s.Get(context.Background(), *namespacedName, configMap)
	if err != nil {
		logger.Error(err, "Failed to read configmap with certificates")
		return nil, false
	}
	return configMap.Data, true
}

func injectCertificates(certsPem []byte, transport *http.Transport, logger logr.Logger) {
	caCertPool := transport.TLSClientConfig.RootCAs
	if caCertPool == nil {
		systemCertPool, err := x509.SystemCertPool()
		if err != nil {
			logger.Error(err, "Failed to load system cert pool")
			caCertPool = x509.NewCertPool()
		} else {
			caCertPool = systemCertPool
		}
	}
	if ok := caCertPool.AppendCertsFromPEM(certsPem); ok {
		transport.TLSClientConfig = &tls.Config{RootCAs: caCertPool}
	}
}
