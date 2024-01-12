// Copyright (c) 2019-2024 Red Hat, Inc.
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
	"testing"

	controller "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"
)

func TestMergeProxyConfigs(t *testing.T) {
	tests := []struct {
		name           string
		operatorConfig *controller.Proxy
		clusterConfig  *controller.Proxy
		expectedConfig *controller.Proxy
	}{
		{
			name: "Uses operatorConfig as base when configured",
			operatorConfig: &controller.Proxy{
				HttpProxy:  pointer.String("test-operator-http"),
				HttpsProxy: pointer.String("test-operator-https"),
				NoProxy:    pointer.String("test-operator-noproxy"),
			},
			clusterConfig: &controller.Proxy{
				HttpProxy:  pointer.String("test-cluster-http"),
				HttpsProxy: pointer.String("test-cluster-https"),
				NoProxy:    pointer.String("test-cluster-noproxy"),
			},
			expectedConfig: &controller.Proxy{
				HttpProxy:  pointer.String("test-operator-http"),
				HttpsProxy: pointer.String("test-operator-https"),
				NoProxy:    pointer.String("test-cluster-noproxy,test-operator-noproxy"),
			},
		},
		{
			name: "Uses clusterConfig when operator not configured",
			operatorConfig: &controller.Proxy{
				HttpProxy:  nil,
				HttpsProxy: nil,
				NoProxy:    nil,
			},
			clusterConfig: &controller.Proxy{
				HttpProxy:  pointer.String("test-cluster-http"),
				HttpsProxy: pointer.String("test-cluster-https"),
				NoProxy:    pointer.String("test-cluster-noproxy"),
			},
			expectedConfig: &controller.Proxy{
				HttpProxy:  pointer.String("test-cluster-http"),
				HttpsProxy: pointer.String("test-cluster-https"),
				NoProxy:    pointer.String("test-cluster-noproxy"),
			},
		},
		{
			name: "Can set empty strings to unset autodetected fields",
			operatorConfig: &controller.Proxy{
				HttpProxy:  pointer.String(""),
				HttpsProxy: pointer.String(""),
				NoProxy:    pointer.String(""),
			},
			clusterConfig: &controller.Proxy{
				HttpProxy:  pointer.String("test-cluster-http"),
				HttpsProxy: pointer.String("test-cluster-https"),
				NoProxy:    pointer.String("test-cluster-noproxy"),
			},
			expectedConfig: &controller.Proxy{},
		},
		{
			name:           "Nil operator proxy uses cluster config proxy",
			operatorConfig: nil,
			clusterConfig: &controller.Proxy{
				HttpProxy:  pointer.String("test-cluster-http"),
				HttpsProxy: pointer.String("test-cluster-https"),
				NoProxy:    pointer.String("test-cluster-noproxy"),
			},
			expectedConfig: &controller.Proxy{
				HttpProxy:  pointer.String("test-cluster-http"),
				HttpsProxy: pointer.String("test-cluster-https"),
				NoProxy:    pointer.String("test-cluster-noproxy"),
			},
		},
		{
			name: "Nil cluster proxy uses operator-configured proxy",
			operatorConfig: &controller.Proxy{
				HttpProxy:  pointer.String("test-operator-http"),
				HttpsProxy: pointer.String("test-operator-https"),
				NoProxy:    pointer.String("test-operator-noproxy"),
			},
			clusterConfig: nil,
			expectedConfig: &controller.Proxy{
				HttpProxy:  pointer.String("test-operator-http"),
				HttpsProxy: pointer.String("test-operator-https"),
				NoProxy:    pointer.String("test-operator-noproxy"),
			},
		},
		{
			name:           "Handles no proxy configured",
			operatorConfig: nil,
			clusterConfig:  nil,
			expectedConfig: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualConfig := MergeProxyConfigs(tt.operatorConfig, tt.clusterConfig)
			assert.Equal(t, tt.expectedConfig, actualConfig)
		})
	}
}
