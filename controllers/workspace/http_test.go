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
	"testing"
	"time"

	controller "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestConfigureHttpClients_InitializesClientsWithNilRoutingConfig(t *testing.T) {
	h := newTestHolder(nil)

	h.ConfigureHttpClients(context.Background(), nil)

	assert.NotNil(t, h.GetHttpClient())
	assert.NotNil(t, h.GetHealthCheckHttpClient())
}

func TestConfigureHttpClients_InitializesClientsWithEmptyRoutingConfig(t *testing.T) {
	h := newTestHolder(nil)

	h.ConfigureHttpClients(context.Background(), &controller.RoutingConfig{})

	assert.NotNil(t, h.GetHttpClient())
	assert.NotNil(t, h.GetHealthCheckHttpClient())
}

func TestConfigureHttpClients_SetsProxyOnBothClients(t *testing.T) {
	h := newTestHolder(nil)

	routingConfig := &controller.RoutingConfig{
		ProxyConfig: &controller.Proxy{
			HttpProxy: pointer.String("http://proxy:8080"),
		},
	}

	h.ConfigureHttpClients(context.Background(), routingConfig)

	httpTransport := getHttpClientTransport(t, h.GetHttpClient())
	assert.NotNil(t, httpTransport.Proxy)

	healthTransport := getHttpClientTransport(t, h.GetHealthCheckHttpClient())
	assert.NotNil(t, healthTransport.Proxy)
}

func TestConfigureHttpClients_LoadsCertificatesFromConfigMap(t *testing.T) {
	certPEM := generateSelfSignedCertPEM(t)
	certCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "tls-certs",
			Namespace:       "cert-ns",
			ResourceVersion: "1",
		},
		Data: map[string]string{
			"ca.crt": string(certPEM),
		},
	}

	h := newTestHolder(certCM)

	routingConfig := &controller.RoutingConfig{
		TLSCertificateConfigmapRef: &controller.ConfigmapReference{
			Name:      "tls-certs",
			Namespace: "cert-ns",
		},
	}

	h.ConfigureHttpClients(context.Background(), routingConfig)

	tlsCfg := getHttpClientTransport(t, h.GetHttpClient()).TLSClientConfig
	assert.NotNil(t, tlsCfg.RootCAs)
}

func TestConfigureHttpClients_SkipsRebuildWhenNothingChanged(t *testing.T) {
	h := newTestHolder(nil)

	routingConfig := &controller.RoutingConfig{
		ProxyConfig: &controller.Proxy{
			HttpProxy: pointer.String("http://proxy:8080"),
		},
	}

	h.ConfigureHttpClients(context.Background(), routingConfig)
	firstClient := h.GetHttpClient()
	firstHealthCheck := h.GetHealthCheckHttpClient()

	h.ConfigureHttpClients(context.Background(), routingConfig)

	assert.Same(t, firstClient, h.GetHttpClient())
	assert.Same(t, firstHealthCheck, h.GetHealthCheckHttpClient())
}

func TestConfigureHttpClients_RebuildsBothWhenProxyChanges(t *testing.T) {
	h := newTestHolder(nil)

	h.ConfigureHttpClients(context.Background(), &controller.RoutingConfig{
		ProxyConfig: &controller.Proxy{
			HttpProxy: pointer.String("http://old-proxy:8080"),
		},
	})
	firstClient := h.GetHttpClient()
	firstHealthCheck := h.GetHealthCheckHttpClient()

	h.ConfigureHttpClients(context.Background(), &controller.RoutingConfig{
		ProxyConfig: &controller.Proxy{
			HttpProxy: pointer.String("http://new-proxy:8080"),
		},
	})

	assert.NotSame(t, firstClient, h.GetHttpClient())
	assert.NotSame(t, firstHealthCheck, h.GetHealthCheckHttpClient())
}

func TestConfigureHttpClients_RebuildOnlyHttpClientWhenCertCMChanges(t *testing.T) {
	certCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "tls-certs",
			Namespace:       "cert-ns",
			ResourceVersion: "1",
		},
		Data: map[string]string{
			"ca.crt": string(generateSelfSignedCertPEM(t)),
		},
	}

	h := newTestHolder(certCM)
	routingConfig := &controller.RoutingConfig{
		TLSCertificateConfigmapRef: &controller.ConfigmapReference{
			Name:      "tls-certs",
			Namespace: "cert-ns",
		},
	}

	h.ConfigureHttpClients(context.Background(), routingConfig)
	firstClient := h.GetHttpClient()
	firstHealthCheck := h.GetHealthCheckHttpClient()

	// Simulate configmap update: read current, update data, write back
	currentCM := &corev1.ConfigMap{}
	require.NoError(t, h.k8s.Get(context.Background(), client.ObjectKeyFromObject(certCM), currentCM))

	currentCM.Data["ca.crt"] = string(generateSelfSignedCertPEM(t))
	require.NoError(t, h.k8s.Update(context.Background(), currentCM))

	h.ConfigureHttpClients(context.Background(), routingConfig)

	assert.NotSame(t, firstClient, h.GetHttpClient())
	assert.Same(t, firstHealthCheck, h.GetHealthCheckHttpClient())
}

func TestConfigureHttpClients_HandlesInvalidCertDataGracefully(t *testing.T) {
	invalidCertCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "bad-certs",
			Namespace:       "cert-ns",
			ResourceVersion: "1",
		},
		Data: map[string]string{
			"ca.crt": "not-a-valid-certificate",
		},
	}

	h := newTestHolder(invalidCertCM)

	routingConfig := &controller.RoutingConfig{
		TLSCertificateConfigmapRef: &controller.ConfigmapReference{
			Name:      "bad-certs",
			Namespace: "cert-ns",
		},
	}

	h.ConfigureHttpClients(context.Background(), routingConfig)

	assert.NotNil(t, h.GetHttpClient())
	assert.NotNil(t, h.GetHealthCheckHttpClient())
}

func TestConfigureHttpClients_HandlesMissingCertConfigMapGracefully(t *testing.T) {
	h := newTestHolder(nil)

	routingConfig := &controller.RoutingConfig{
		TLSCertificateConfigmapRef: &controller.ConfigmapReference{
			Name:      "nonexistent",
			Namespace: "cert-ns",
		},
	}

	h.ConfigureHttpClients(context.Background(), routingConfig)

	assert.NotNil(t, h.GetHttpClient())
	assert.NotNil(t, h.GetHealthCheckHttpClient())
}

func newTestHolder(certCM *corev1.ConfigMap) *DefaultHttpClientsHolder {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	h := &DefaultHttpClientsHolder{
		logger:         zap.New(zap.UseDevMode(true)),
		systemCertPool: x509.NewCertPool(),
	}

	if certCM != nil {
		h.k8s = fake.NewClientBuilder().
			WithScheme(scheme).
			WithRuntimeObjects(certCM).
			Build()
	} else {
		h.k8s = fake.NewClientBuilder().
			WithScheme(scheme).
			Build()
	}

	return h
}

func generateSelfSignedCertPEM(t *testing.T) []byte {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-ca"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
}

func getHttpClientTransport(t *testing.T, c *http.Client) *http.Transport {
	t.Helper()

	transport, ok := c.Transport.(*http.Transport)
	require.True(t, ok)

	return transport
}
