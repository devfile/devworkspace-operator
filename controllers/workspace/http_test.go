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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	controller "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type TestHttpClientsFactory struct {
	client                *http.Client
	healthCheckHttpClient *http.Client
}

func (t *TestHttpClientsFactory) GetHttpClient(_ context.Context, _ *controller.RoutingConfig) *http.Client {
	return t.client
}

func (t *TestHttpClientsFactory) GetHealthCheckHttpClient(_ *controller.RoutingConfig) *http.Client {
	return t.healthCheckHttpClient
}

func SetupHttpClientsForTesting(client *http.Client) {
	httpClientsFactory = &TestHttpClientsFactory{
		client:                client,
		healthCheckHttpClient: client,
	}
}

type getClientFunc func(factory HttpClientsFactory, routingConfig *controller.RoutingConfig) *http.Client

func TestGetHttpClient(t *testing.T) {
	getClient := func(
		f HttpClientsFactory,
		routingConfig *controller.RoutingConfig,
	) *http.Client {
		return f.GetHttpClient(context.Background(), routingConfig)
	}

	runCommonClientTests(t, getClient)

	t.Run("rebuilds client when certs changes", func(t *testing.T) {
		factory := newTestFactory(t,
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-certs-1",
					Namespace: "default",
				},
				Data: map[string]string{"ca.crt": generateTestCACert(t)},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-certs-2",
					Namespace: "default",
				},
				Data: map[string]string{"ca.crt": generateTestCACert(t)},
			})
		routingConfig1 := routingConfigWithCerts("test-certs-1", "default")
		routingConfig2 := routingConfigWithCerts("test-certs-2", "default")

		client1 := factory.GetHttpClient(context.Background(), routingConfig1)

		assert.NotNil(t, client1.Transport.(*http.Transport).TLSClientConfig.RootCAs)

		client2 := factory.GetHttpClient(context.Background(), routingConfig2)

		assert.NotNil(t, client2.Transport.(*http.Transport).TLSClientConfig.RootCAs)
		assert.NotSame(t, client1, client2)

		client3 := factory.GetHttpClient(context.Background(), nil)

		assert.NotSame(t, client2, client3)
		assert.Nil(t, client3.Transport.(*http.Transport).TLSClientConfig.RootCAs)
	})
}

func TestGetHealthCheckHttpClient(t *testing.T) {
	getClient := func(
		f HttpClientsFactory,
		rc *controller.RoutingConfig,
	) *http.Client {
		return f.GetHealthCheckHttpClient(rc)
	}

	runCommonClientTests(t, getClient)
}

func runCommonClientTests(t *testing.T, getClient getClientFunc) {
	t.Run("returns non-nil client", func(t *testing.T) {
		factory := newTestFactory(t)

		client := getClient(factory, nil)

		require.NotNil(t, client)
	})

	t.Run("caches client on repeated calls", func(t *testing.T) {
		factory := newTestFactory(t)

		client1 := getClient(factory, nil)
		client2 := getClient(factory, nil)

		assert.Same(t, client1, client2)
	})

	t.Run("rebuilds client when proxy changes", func(t *testing.T) {
		factory := newTestFactory(t)
		routingConfig1 := routingConfigWithProxy("http://proxy:80", "", "")
		routingConfig2 := routingConfigWithProxy("http://proxy:90", "", "")

		client1 := getClient(factory, routingConfig1)

		assert.NotNil(t, client1.Transport.(*http.Transport).Proxy)

		client2 := getClient(factory, routingConfig2)

		assert.NotNil(t, client2.Transport.(*http.Transport).Proxy)
		assert.NotSame(t, client1, client2)

		client3 := getClient(factory, nil)

		assert.NotSame(t, client2, client3)

		// Default proxy config is not nil
		assert.NotNil(t, client3.Transport.(*http.Transport).Proxy)
	})

	t.Run("safe for concurrent access", func(t *testing.T) {
		factory := newTestFactory(t)
		routingConfigs := []*controller.RoutingConfig{
			routingConfigWithProxy("http://proxy:80", "", ""),
			routingConfigWithProxy("http://proxy:90", "", ""),
		}

		var wg sync.WaitGroup
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				routingConfig := routingConfigs[idx%len(routingConfigs)]
				client := getClient(factory, routingConfig)

				assert.NotNil(t, client)
				assert.NotNil(t, client.Transport.(*http.Transport).Proxy)
			}(i)
		}
		wg.Wait()
	})
}

func generateTestCACert(t *testing.T) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func newTestFactory(t *testing.T, objs ...runtime.Object) *DefaultHttpClientsFactory {
	t.Helper()

	systemCertPool, err := x509.SystemCertPool()
	require.NoError(t, err)

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()

	return &DefaultHttpClientsFactory{
		k8s:            k8sClient,
		logger:         zap.New(zap.UseDevMode(true)),
		systemCertPool: systemCertPool,
	}
}

func routingConfigWithProxy(httpProxy, httpsProxy, noProxy string) *controller.RoutingConfig {
	return &controller.RoutingConfig{
		ProxyConfig: &controller.Proxy{
			HttpProxy:  ptr.To(httpProxy),
			HttpsProxy: ptr.To(httpsProxy),
			NoProxy:    ptr.To(noProxy),
		},
	}
}

func routingConfigWithCerts(name, namespace string) *controller.RoutingConfig {
	return &controller.RoutingConfig{
		TLSCertificateConfigmapRef: &controller.ConfigmapReference{
			Name:      name,
			Namespace: namespace,
		},
	}
}
