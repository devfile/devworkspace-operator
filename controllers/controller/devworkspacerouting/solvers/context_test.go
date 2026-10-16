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

package solvers

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config"
)

func TestDiscoverableServiceLookupPreservesCancellation(t *testing.T) {
	previousConfig := config.GetGlobalConfig()
	config.SetGlobalConfigForTesting(&controllerv1alpha1.OperatorConfiguration{Routing: &controllerv1alpha1.RoutingConfig{ClusterHostSuffix: "example.test"}})
	t.Cleanup(func() { config.SetGlobalConfigForTesting(previousConfig) })
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	routing := &controllerv1alpha1.DevWorkspaceRouting{Spec: controllerv1alpha1.DevWorkspaceRoutingSpec{Endpoints: map[string]controllerv1alpha1.EndpointList{
		"main": {{Name: "api", TargetPort: 8080, Exposure: controllerv1alpha1.InternalEndpointExposure, Attributes: controllerv1alpha1.Attributes{}.PutBoolean("discoverable", true)}},
	}}}
	meta := DevWorkspaceMetadata{DevWorkspaceId: "mine", Namespace: "ns"}
	for _, entry := range []struct {
		name string
		run  func(context.Context, client.Client) ([]corev1.Service, error)
	}{
		{"helper", func(ctx context.Context, cl client.Client) ([]corev1.Service, error) {
			return GetDiscoverableServicesForEndpoints(ctx, routing.Spec.Endpoints, meta, cl)
		}},
		{"cluster", func(ctx context.Context, cl client.Client) ([]corev1.Service, error) {
			objects, err := NewClusterSolver(cl, false).GetSpecObjects(ctx, routing, meta)
			return objects.Services, err
		}},
		{"basic", func(ctx context.Context, cl client.Client) ([]corev1.Service, error) {
			objects, err := NewBasicSolver(cl).GetSpecObjects(ctx, routing, meta)
			return objects.Services, err
		}},
	} {
		for _, alreadyCanceled := range []bool{true, false} {
			name := "during lookup"
			if alreadyCanceled {
				name = "already canceled"
			}
			t.Run(entry.name+"/"+name, func(t *testing.T) {
				deadline := time.Now().Add(time.Hour)
				ctx, cancel := context.WithDeadline(context.WithValue(context.Background(), lookupContextKey{}, "lookup"), deadline)
				defer cancel()
				if alreadyCanceled {
					cancel()
				}
				cl := &cancelingLookupClient{Client: fake.NewClientBuilder().WithScheme(scheme).Build(), cancel: cancel}
				services, err := entry.run(ctx, cl)
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("want cancellation error, got %v", err)
				}
				if len(services) != 0 {
					t.Fatalf("canceled lookup returned Services: %#v", services)
				}
				if cl.received == nil || cl.received.Value(lookupContextKey{}) != "lookup" {
					t.Fatal("lookup lost caller context value")
				}
				if actual, ok := cl.received.Deadline(); !ok || !actual.Equal(deadline) {
					t.Fatalf("lookup lost caller deadline: %v, present=%t", actual, ok)
				}
			})
		}
	}
}

type lookupContextKey struct{}

type cancelingLookupClient struct {
	client.Client
	cancel   context.CancelFunc
	received context.Context
}

func (c *cancelingLookupClient) Get(ctx context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
	c.received = ctx
	c.cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	return errors.New("Service lookup did not observe caller cancellation")
}
