// Copyright (c) 2019-2023 Red Hat, Inc.
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
	"fmt"

	controller "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	configv1 "github.com/openshift/api/config/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	openshiftClusterProxyName = "cluster"
)

// GetClusterProxyConfig reads a proxy configuration from the "cluster" proxies.config.openshift.io on
// OpenShift. If running in a non-OpenShift cluster, returns (nil, nil). If the cluster proxy is empty, returns
// (nil, nil)
func GetClusterProxyConfig(nonCachedClient crclient.Client) (*controller.Proxy, error) {
	if !infrastructure.IsOpenShift() {
		return nil, nil
	}
	proxy := &configv1.Proxy{}
	err := nonCachedClient.Get(context.Background(), types.NamespacedName{Name: openshiftClusterProxyName}, proxy)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			// Should never happen as OpenShift cluster proxy is always present
			return nil, nil
		}
		return nil, err
	}

	if proxy.Status.HTTPProxy == "" && proxy.Status.HTTPSProxy == "" && proxy.Status.NoProxy == "" {
		return nil, nil
	}

	proxyConfig := &controller.Proxy{
		HttpProxy:  &proxy.Status.HTTPProxy,
		HttpsProxy: &proxy.Status.HTTPSProxy,
		NoProxy:    &proxy.Status.NoProxy,
	}

	return proxyConfig, nil
}

// MergeProxyConfigs merges proxy configurations from the operator and the cluster and merges them, with the
// operator configuration taking precedence. Accepts nil arguments. If both arguments are nil, returns nil.
func MergeProxyConfigs(operatorConfig, clusterConfig *controller.Proxy) *controller.Proxy {
	if clusterConfig == nil {
		return removeEmptyStrings(operatorConfig)
	}
	if operatorConfig == nil {
		return removeEmptyStrings(clusterConfig)
	}

	mergedProxy := &controller.Proxy{
		HttpProxy:  operatorConfig.HttpProxy,
		HttpsProxy: operatorConfig.HttpsProxy,
		NoProxy:    operatorConfig.NoProxy,
	}

	if mergedProxy.HttpProxy == nil {
		mergedProxy.HttpProxy = clusterConfig.HttpProxy
	}
	if mergedProxy.HttpsProxy == nil {
		mergedProxy.HttpsProxy = clusterConfig.HttpsProxy
	}
	if mergedProxy.NoProxy == nil {
		mergedProxy.NoProxy = clusterConfig.NoProxy
	} else if *mergedProxy.NoProxy != "" {
		// Merge noProxy fields, joining with a comma
		if clusterConfig.NoProxy != nil {
			noProxy := fmt.Sprintf("%s,%s", *clusterConfig.NoProxy, *operatorConfig.NoProxy)
			mergedProxy.NoProxy = &noProxy
		}
	}

	return removeEmptyStrings(mergedProxy)
}

// removeEmptyStrings is a utility function for removing empty fields from a proxy configuration. This is required
// to allow overriding
func removeEmptyStrings(proxyConfig *controller.Proxy) *controller.Proxy {
	if proxyConfig == nil {
		return nil
	}

	updated := &controller.Proxy{}
	if proxyConfig.HttpProxy != nil && *proxyConfig.HttpProxy != "" {
		updated.HttpProxy = proxyConfig.HttpProxy
	}
	if proxyConfig.HttpsProxy != nil && *proxyConfig.HttpsProxy != "" {
		updated.HttpsProxy = proxyConfig.HttpsProxy
	}
	if proxyConfig.NoProxy != nil && *proxyConfig.NoProxy != "" {
		updated.NoProxy = proxyConfig.NoProxy
	}
	return updated
}
