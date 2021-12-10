// Copyright (c) 2019-2021 Red Hat, Inc.
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

package proxy

import (
	"context"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/go-logr/logr"
	configv1 "github.com/openshift/api/config/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	openshiftClusterProxyName = "cluster"
)

type Proxy struct {
	// HttpProxy is the URL of the proxy for HTTP requests, in the format http://USERNAME:PASSWORD@SERVER:PORT/
	HttpProxy string
	// HttpsProxy is the URL of the proxy for HTTPS requests, in the format http://USERNAME:PASSWORD@SERVER:PORT/
	HttpsProxy string
	// NoProxy is a comma-separated list of hostnames and/or CIDRs for which the proxy should not be used. Ignored
	// when HttpProxy and HttpsProxy are unset
	NoProxy string
}

func GetOpenShiftClusterProxyConfig(nonCachedClient crclient.Client, log logr.Logger) (*Proxy, error) {
	if !infrastructure.IsOpenShift() {
		return nil, nil
	}
	proxy := &configv1.Proxy{}
	err := nonCachedClient.Get(context.Background(), types.NamespacedName{Name: openshiftClusterProxyName}, proxy)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	proxyConfig := &Proxy{
		HttpProxy:  proxy.Status.HTTPProxy,
		HttpsProxy: proxy.Status.HTTPSProxy,
		NoProxy:    proxy.Status.NoProxy,
	}
	log.Info("Read proxy configuration", "config", proxyConfig)

	return proxyConfig, nil
}
