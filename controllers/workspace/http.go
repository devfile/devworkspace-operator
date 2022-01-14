// Copyright (c) 2019-2022 Red Hat, Inc.
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
	"crypto/tls"
	"net/http"
	"net/url"

	"github.com/devfile/devworkspace-operator/pkg/config"
	"golang.org/x/net/http/httpproxy"
)

var (
	httpClient            *http.Client
	healthCheckHttpClient *http.Client
)

func setupHttpClients() {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	healthCheckTransport := http.DefaultTransport.(*http.Transport).Clone()
	healthCheckTransport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	if config.Routing != nil && config.Routing.ProxyConfig != nil {
		proxyConf := httpproxy.Config{
			HTTPProxy:  config.Routing.ProxyConfig.HttpProxy,
			HTTPSProxy: config.Routing.ProxyConfig.HttpsProxy,
			NoProxy:    config.Routing.ProxyConfig.NoProxy,
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
	}
}
