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
	"net/http"

	controller "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
)

type TestHttpsClientHolder struct {
	client                *http.Client
	healthCheckHttpClient *http.Client
}

func (t *TestHttpsClientHolder) GetHttpClient() *http.Client {
	return t.client
}

func (t *TestHttpsClientHolder) GetHealthCheckHttpClient() *http.Client {
	return t.healthCheckHttpClient
}

func (t *TestHttpsClientHolder) ConfigureHttpClients(_ *controller.RoutingConfig) {}

func SetupHttpClientsForTesting(client *http.Client) {
	httpClientsHolder = &TestHttpsClientHolder{
		client:                client,
		healthCheckHttpClient: client,
	}
}
